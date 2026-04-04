// this is a gpt generated TUI that I put as a placeholder lowkenuinely
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"memcommands/core"
)

type model struct {
	count int
}

func main() {
	lines, err := core.GetHistoryLines();
	reader := bufio.NewReader(os.Stdin);
	
	if err != nil {
		fmt.Println(err) 
		return
	}

	fmt.Println("Enter input: ");

	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("There was an error reading the input: ")
		return
	}

	input = strings.TrimSpace(input)
	fmt.Printf("You entered: %s\n", input)
	
	res := core.GetFuzzyScoreList(lines, input);

	for _, val := range res {
		fmt.Println(val)
	}
}
