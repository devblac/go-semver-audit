// Package main is a sample user project
package main

import (
	"fmt"

	"example.com/oldlib"
)

func main() {
	// Uses ParseConfig - signature will change in v2
	config, err := oldlib.ParseConfig("config.yaml")
	if err != nil {
		panic(err)
	}
	fmt.Println(config.Name)

	// Uses OldHelper - will be removed in v2
	helper := oldlib.OldHelper()
	fmt.Println(helper)

	// Uses Transform - signature unchanged
	result := oldlib.Transform("test")
	fmt.Println(result)
}

