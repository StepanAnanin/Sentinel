package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"sentinel/packages/infrastructure/DB"
	"sentinel/packages/infrastructure/cache"
	"sentinel/packages/infrastructure/config"
	"sentinel/packages/presentation/api/http/router"
	"sentinel/packages/util"

	emongo "github.com/StepanAnanin/EssentialMongoDB"
	"github.com/StepanAnanin/weaver"
)

var logo = `

  ███████╗ ███████╗ ███╗   ██╗ ████████╗ ██╗ ███╗   ██╗ ███████╗ ██╗
  ██╔════╝ ██╔════╝ ████╗  ██║ ╚══██╔══╝ ██║ ████╗  ██║ ██╔════╝ ██║
  ███████╗ █████╗   ██╔██╗ ██║    ██║    ██║ ██╔██╗ ██║ █████╗   ██║
  ╚════██║ ██╔══╝   ██║╚██╗██║    ██║    ██║ ██║╚██╗██║ ██╔══╝   ██║
  ███████║ ███████╗ ██║ ╚████║    ██║    ██║ ██║ ╚████║ ███████╗ ███████╗
  ╚══════╝ ╚══════╝ ╚═╝  ╚═══╝    ╚═╝    ╚═╝ ╚═╝  ╚═══╝ ╚══════╝ ╚══════╝

`

func main() {
	ver := "1.1.0.0"

	// Program wasn't run and/or tested on Windows.
	// (Probably it will work, but required minor code modifications)
	if runtime.GOOS != "linux" {
		log.Fatalln("[ CRITICAL ERROR ] OS is not supported. This program can be used only on Linux.")
	}

	DB.Database.Connect()

	emongo.Config.DefaultQueryTimeout = config.DB.DefaultQueryTimeout

	defer DB.Database.Disconnect()

	cache.Init()

	log.Println("[ SERVER ] Initializng router...")

	Router := router.Create()

	http.Handle("/", Router)

	log.Println("[ SERVER ] Initializng router: OK")

	util.ClearTerminal()

	fmt.Print(logo)

	fmt.Printf("  Authentication/authorization service (v%s)\n", ver)

	fmt.Println("  Mady by Stepan Ananin (xrf844@gmail.com)")

	fmt.Printf("  Listening on port: %s\n\n", config.HTTP.Port)

	if config.Debug.Enabled {
		fmt.Printf("[ WARNING ] Debug mode enabled. Some functions may work different and return unexpected results. \n\n")
	}

	weaver.Settings.DefaultOrigin = config.HTTP.AllowedOrigin

	if err := http.ListenAndServe(":"+config.HTTP.Port, Router); err != nil {
		log.Println("[ CRITICAL ERROR ] Server error has occurred, the program will stop")

		DB.Database.Disconnect()

		panic(err)
	}
}
