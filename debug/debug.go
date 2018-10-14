package debug

import (
	"fmt"
	"sync/atomic"
)

var aid *atomic.Value

func init() {
	aid = new(atomic.Value)
	aid.Store(1)
}

func Println(a ...interface{}) {
	id := aid.Load().(int)
	aid.Store(id + 1)

	fmt.Println(append([]interface{}{id}, a...)...)
}

func PrintlnWithQuote(a ...interface{}) {
	id := aid.Load().(int)
	aid.Store(id + 1)

	fmt.Println(append([]interface{}{id, "=<"}, append(a, ">=")...)...)
}

/*

看不懂：
不懂？？？


学习：
学习：xxx
*/
