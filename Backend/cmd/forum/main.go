package main

import (
	"flag"
	"forum/internal/app"
	"forum/internal/common"
)

var (
	port int
	path string
)

func main() {
	flag.IntVar(&port, "port", 8081, "Specify the app port.")
	flag.StringVar(&path, "path", "./dataBase.db", "Specify path to database")
	flag.Parse()

	a := new(app.App)
	err := a.Run(port, path)
	if err != nil {
		panic(err)
	}
	common.InfoLogger.Println("Application runs")
}
