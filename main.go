package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"pesatu/app/chat"
	"pesatu/auth"
	"pesatu/components/contacts"
	"pesatu/components/images"
	"pesatu/components/user"
	"pesatu/components/userprofile"
	"pesatu/utils"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/juju/ratelimit"
	log "github.com/pion/ion-sfu/pkg/logger"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type logC struct {
	Config log.GlobalConfig `mapstructure:"log"`
}

var (
	server         *gin.Engine
	ctx            context.Context
	Addr           string
	DevMode        int
	verbosityLevel int
	logConfig      logC
	logger         = log.New()
	limiter        *ratelimit.Bucket
	Env            string
)

func showHelp() {
	fmt.Printf("Usage:%s {params}\n", os.Args[0])
	fmt.Println("      -a {listen addr}")
	fmt.Println("      -h (show help info)")
	fmt.Println("      -v {0-2} (verbosity level, default 0)")
	fmt.Println("      -dev {0/1} (developer mode, default disabled (0))")
	fmt.Println("      -env .env file location path, default current")
}

func parse() bool {
	flag.StringVar(&Addr, "a", ":7000", "address to use")
	flag.IntVar(&verbosityLevel, "v", -1, "verbosity level, higher value - more logs")
	flag.IntVar(&DevMode, "dev", 0, "dev mode to enable/disable cross origin")
	flag.StringVar(&Env, "env", "", ".env file location path")
	help := flag.Bool("h", false, "help info")
	flag.Parse()

	if *help {
		return false
	}
	return true
}

func readEnv() {
	env := ".env"
	if strings.HasSuffix(Env, ".env") {
		env = Env
	} else {
		env = fmt.Sprintf("%s.env", Env)
	}

	// Open the .env file
	file, err := os.Open(env)
	if err != nil {
		utils.Log().Error(err, "error opening .env file")
		return
	}
	defer file.Close()

	// Read the contents of the .env file
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Split the line into a variable name and value
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)

		// Set the environment variable
		os.Setenv(parts[0], parts[1])
	}

	// Check if there was an error during scanning
	if err := scanner.Err(); err != nil {
		utils.Log().Error(err, "error reading .env file")
		return
	}

	// Use the environment variables in your code
	hmacsecret := os.Getenv("HMACSECRET")
	if len(hmacsecret) == 32 {
		utils.Log().V(2).Info("using .env hmacsecret")
		auth.SetHmacSecret(hmacsecret)
	}
}

func main() {
	if !parse() {
		showHelp()
		os.Exit(-1)
	}

	// Check that the -v is not set (default -1)
	if verbosityLevel < 0 {
		verbosityLevel = logConfig.Config.V
	}

	logger.Info(fmt.Sprintf("verbosity level is: %d", verbosityLevel))
	log.SetGlobalOptions(log.GlobalConfig{V: verbosityLevel})

	utils.InitLogger(logger)

	readEnv()

	ctx = context.TODO()
	// // Set up context and options for connecting to MongoDB
	// ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	// defer cancel()

	// Connect to MongoDB
	mongoconn := options.Client().ApplyURI("mongodb://root:example@mongo:27017")
	mongoclient, err := mongo.NewClient(mongoconn)
	if err != nil {
		panic(err)
	}

	err = mongoclient.Connect(ctx)
	if err != nil {
		panic(err)
	}
	defer mongoclient.Disconnect(ctx)

	if err := mongoclient.Ping(ctx, readpref.Primary()); err != nil {
		panic(err)
	}

	logger.Info("MongoDB successfully connected...")

	server = gin.Default()
	limiter := ratelimit.NewBucketWithRate(100, 100)

	if DevMode > 0 {
		// Enable CORS with the withCredentials option
		server.Use(cors.New(cors.Config{
			AllowOrigins:     []string{"http://localhost:3000"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
			AllowHeaders:     []string{"Content-Type", "Authorization", "credentials", "Origin"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowWebSockets:  true,
			AllowCredentials: true,
		}))
	}

	server.GET("/", func(c *gin.Context) {
		if limiter.TakeAvailable(1) == 0 {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}

		c.Redirect(http.StatusMovedPermanently, "/app/")
	})
	server.Static("/static", "./public/static")
	server.Static("/app", "./public")

	server.Use(auth.AuthMiddleware())

	UserRouteController := user.NewUserRoute(mongoclient, ctx, logger, limiter)
	UserRouteController.InitRouteTo(server)

	UploadImageRouteCtr := images.NewUploadImageRoute(mongoclient, ctx, logger, limiter, UserRouteController.GetUserService())
	UploadImageRouteCtr.InitRouteTo(server)

	//for testing
	server.Use(DelayMiddleware(1 * time.Second))

	ProfileRouteController := userprofile.NewProfileRoute(mongoclient, ctx, logger, limiter, UserRouteController.GetUserService())
	ProfileRouteController.InitRouteTo(server)

	ContactRouteController := contacts.NewContactRoute(mongoclient, ctx, logger, limiter, UserRouteController.GetUserService())
	ContactRouteController.InitRouteTo(server)

	//app:
	wsServer := chat.NewWebsocketServer(mongoclient, ctx)
	wsServer.InitRouteTo(server, ContactRouteController.GetContactService(), DevMode)
	go wsServer.Run()

	// Use the redirectToAppMiddleware middleware to wrap the handler
	server.Use(redirectToAppMiddleware())

	server.Run(Addr)
}

func redirectToAppMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		u, err := url.Parse(c.Request.URL.String())
		if err != nil {
			logger.Error(err, "redirecting errror")
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Get the path and query parameters from the original request
		path := u.Path
		if strings.Contains(path, ":") {
			logger.Error(fmt.Errorf("path unsupported %s", path), "redirecting errror")
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		path = strings.TrimPrefix(path, "/")

		params := c.Request.URL.Query()
		queryString := params.Encode()
		if queryString != "" {
			queryString = "?" + queryString
		}

		// Construct the target URL using "/app/#" so it can be handled using FE
		targetURL := "/app/#" + path + queryString
		// Redirect to the target URL
		c.Redirect(http.StatusMovedPermanently, targetURL)
	}
}

func DelayMiddleware(duration time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		time.Sleep(duration)
		c.Next()
	}
}
