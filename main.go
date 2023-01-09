package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"pesatu/images"
	"pesatu/user"
	"pesatu/userprofile"
	"strings"

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
	verbosityLevel int
	logConfig      logC
	logger         = log.New()
	limiter        *ratelimit.Bucket
)

func showHelp() {
	fmt.Printf("Usage:%s {params}\n", os.Args[0])
	fmt.Println("      -a {listen addr}")
	fmt.Println("      -h (show help info)")
	fmt.Println("      -v {0-2} (verbosity level, default 0)")
}

func parse() bool {
	flag.StringVar(&Addr, "a", ":7000", "address to use")
	flag.IntVar(&verbosityLevel, "v", -1, "verbosity level, higher value - more logs")
	help := flag.Bool("h", false, "help info")
	flag.Parse()

	if *help {
		return false
	}
	return true
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
	// Enable CORS with the withCredentials option
	server.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:7000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "credentials"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	server.GET("/", func(c *gin.Context) {
		if limiter.TakeAvailable(1) == 0 {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}

		c.Redirect(http.StatusMovedPermanently, "/app/")
	})
	server.Static("/static", "./public/static")
	server.Static("/app", "./public")

	UserRouteController := user.NewUserRoute(mongoclient, ctx, logger, limiter)
	UserRouteController.InitRouteTo(server)

	UploadImageRouteCtr := images.NewUploadImageRoute(mongoclient, ctx, logger, limiter)
	UploadImageRouteCtr.InitRouteTo(server)

	ProfileRouteController := userprofile.NewProfileRoute(mongoclient, ctx, logger, limiter, UserRouteController.GetUserService())
	ProfileRouteController.InitRouteTo(server)

	// Use the redirectToAppMiddleware middleware to wrap the handler
	server.Use(redirectToAppMiddleware())

	server.Run(Addr)
}

func redirectToAppMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		u, err := url.Parse(c.Request.URL.String())
		if err != nil {
			logger.Error(err, "redirectinging errror")
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Get the path and query parameters from the original request
		path := u.Path
		if strings.Contains(path, ":") {
			logger.Error(fmt.Errorf("path unsupported %s", path), "redirectinging errror")
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
