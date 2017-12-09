package main

import (
	"fmt"
	"time"
)

func main() {
	ch := make(chan string, 1)

	go func() {
		for m := range ch {
			fmt.Println("processed:", m)
			time.Sleep(2 * time.Second)
		}
		fmt.Println("循环结束")
	}()

	ch <- "cmd.1"
	ch <- "cmd.2" //won't be processed
	close(ch)
	// ch <- "cmd.3" //won't be processed
	time.Sleep(5 * time.Second)
}
