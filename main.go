package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"pesatu/user"

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
	addr           string
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
	flag.StringVar(&addr, "a", ":7000", "address to use")
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

	// Connect to MongoDB
	mongoconn := options.Client().ApplyURI("mongodb://root:example@mongo:27017")
	mongoclient, err := mongo.NewClient(mongoconn) //mongo.Connect(ctx, mongoconn)
	if err != nil {
		panic(err)
	}

	err = mongoclient.Connect(ctx)
	if err != nil {
		panic(err)
	}

	if err := mongoclient.Ping(ctx, readpref.Primary()); err != nil {
		panic(err)
	}

	fmt.Println("MongoDB successfully connected...")

	server = gin.Default()
	limiter := ratelimit.NewBucketWithRate(100, 100)
	// 👇 Add the Post Service, Controllers and Routes
	UserRouteController := user.NewUserControllerRoute(mongoclient, ctx, logger, limiter)
	UserRouteController.InitRouteTo(server)

	server.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/ping/")
	})
	server.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	server.Run(addr)
}
