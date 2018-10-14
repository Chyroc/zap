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
	l, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	l.Info("hi",

		zap.String("string", "string"),
		zap.Bool("bool", true),
		zap.Int("int", 10),
		zap.Time("time", time.Now()),
		zap.ByteString("bytes", []byte("bytes")),
		zap.Any("any-1", map[string]interface{}{"1": "2", "3": 3, "4": false}),
		zap.Any("any-2", Book{Name: "换行", Author: []string{"韩寒", "郭敬明"}}),
	)
}
