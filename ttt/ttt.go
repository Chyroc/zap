package main

import (
	"go.uber.org/zap"
	"time"
)

type Book struct {
	Name   string
	Author []string
}

func main() {

	var err error
	l, err := zap.NewProduction()
	//l, err := zap.NewDevelopment()
	//l := zap.NewExample()
	if err != nil {
		panic(err)
	}

	l.With(zap.Namespace("ns")).Info("hi",
		zap.String("string", "string"),
		zap.Bool("bool", true),
		zap.Int("int", 10),
		zap.Time("time", time.Now()),
		zap.ByteString("bytes", []byte("bytes")),
		zap.Any("any-1", map[string]interface{}{"1": "2", "3": 3, "4": false}),
		zap.Any("any-2", Book{Name: "换行", Author: []string{"韩寒", "郭敬明"}}),
		zap.Strings("array", []string{"1", "2"}),
		zap.Time("time", time.Now()),
	)
}
