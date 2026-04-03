// this is a gpt generated TUI that I put as a placeholder lowkenuinely
package main

import (
	"fmt"
	"memcommands/core"
)

type model struct {
	count int
}

func main() {
	lines, err := core.GetHistoryLines();
	
	if err != nil {
		fmt.Println(err) 
		return
	}

	fmt.Println(lines);
}
