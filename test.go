package main

import "fmt"

func main() {
	mp := make([]int, 3)

	mp[0] = 1
	mp[2] = 3
	mp[1] = 2

	for i, v := range mp {
		fmt.Println(i, v)
	}
}
