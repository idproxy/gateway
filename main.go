package main

import (
	"fmt"
	"net/http"

	"github.com/idproxy/gateway/pkg/gateway2"
)

var db = make(map[string]string)

func main() {
	r := gateway2.Default()

	// Ping test
	r.GET("/", func(gctx *gateway2.Context) {
		gctx.String(http.StatusOK, "pang")
	})

	// Ping test

	r.GET("/ping", func(gctx *gateway2.Context) {
		gctx.String(http.StatusOK, "pong")
	})

	// Ping test
	r.GET("/ping/pong/pang", func(gctx *gateway2.Context) {
		gctx.String(http.StatusOK, "ping.pong.pang")
	})

	routes := r.Group("/test")
	routes.GET("/test1", func(gctx *gateway2.Context) {
		gctx.JSON(http.StatusOK, "test1")
	})
	routes.GET("/test2", func(gctx *gateway2.Context) {
		gctx.JSON(http.StatusOK, "test2")
	})

	// Get user value
	r.GET("/user/:name", func(gctx *gateway2.Context) {
		fmt.Println(gctx.Params)
		user := gctx.Params.ByName("name")
		fmt.Println(user)
		value, ok := db[user]
		if ok {
			gctx.JSON(http.StatusOK, map[string]any{"user": user, "value": value})
		} else {
			gctx.JSON(http.StatusOK, map[string]any{"user": user, "status": "no value"})
		}
	})

	r.POST("/user/:name", func(gctx *gateway2.Context) {
		user := gctx.Params.ByName("name")
		fmt.Println(user)
		db[user] = "ok"
		gctx.JSON(http.StatusOK, "")
	})

	// Get user value
	r.GET("/user/:name/:provider", func(gctx *gateway2.Context) {
		provider := gctx.Params.ByName("provider")
		value, ok := db[provider]
		if ok {
			gctx.JSON(http.StatusOK, map[string]any{"provider": provider, "value": value})
		} else {
			gctx.JSON(http.StatusOK, map[string]any{"provider": provider, "status": "no value"})
		}
	})

	/*
		r.GET("/user/:name/:surname/status", func(gctx *gateway2.Context) {
			name := gctx.Params.ByName("name")
			surname := gctx.Params.ByName("surname")
			fmt.Println(name)
			fmt.Println(surname)
			gctx.JSON(http.StatusOK, "")
		})
	*/

	r.GET("/a/////", func(gctx *gateway2.Context) {
		gctx.String(http.StatusOK, "pong")
	})

	r.PrintMethodTree()

	r.Run(":8888")

}
