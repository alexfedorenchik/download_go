package ui

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type NamedOption interface {
	GetName() string
}

type HasUserChoise interface {
	GetDescription() string
	GetNamedData() []NamedOption
}

func ProcessUserInput(data HasUserChoise, isMultiChoice bool) []int {
	printChoice(data, isMultiChoice)

	for {
		//scan input line
		raw, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
		if err != nil {
			fmt.Println("Unable to scan input. Please repeat.")
			printChoice(data, isMultiChoice)
			continue
		}
		raws := string(raw[0:len(raw)-1])

		//split if mutlivalue
		var splitted []string
		if isMultiChoice {
			splitted = strings.Fields(raws)
		} else {
			splitted = []string{raws}
		}

		//parse to numbers
		parsed, err := strToNum(splitted)
		if err != nil {
			fmt.Printf("Invalid numbers %v. Please repeat.\n", err)
			printChoice(data, isMultiChoice)
			continue
		}

		//check range
		if _, err = checkRange(parsed, len(data.GetNamedData())); err != nil {
			fmt.Printf("Numbers %v are out of range. Please repeat.\n", err)
			printChoice(data, isMultiChoice)
			continue
		}

		//check for 0 value
		checkForInterruption(parsed)

		//check for empty
		if len(parsed) == 0 {
			fmt.Println("Input is empty. Please repeat.")
			continue
		}

		//resolve duplicates
		parsed = removeDuplicates(parsed)

		return parsed
	}
}

func removeDuplicates(ints []int) []int {
	check := make(map[int]bool)
	for _, val := range ints {
		check[val] = true
	}
	res := make([]int, 0)
	for k := range check {
		res = append(res, k)
	}
	return res
}

func checkForInterruption(in []int) {
	for _, num := range in {
		if num == 0 {
			log.Fatalf("Interrupted by user")
		}
	}
}

func checkRange(in []int, max int) (bool, error) {
	erl := make([]string, 0)
	for _, num := range in {
		if num > max || num < 0 {
			erl = append(erl, strconv.Itoa(num))
		}
	}
	if len(erl) > 0 {
		return false, fmt.Errorf("%v", strings.Join(erl, ", "))
	} else {
		return true, nil
	}
}

func strToNum(in []string) ([]int, error) {
	out := make([]int, len(in))
	erl := make([]string, 0)
	for i, str := range in {
		num, err := strconv.Atoi(str)
		if err != nil {
			log.Printf("Failed to parse %v due to %v", str, err)
			erl = append(erl, str)
			continue
		}
		out[i] = num
	}
	if len(erl) > 0 {
		return nil, fmt.Errorf("%v", strings.Join(erl, ", "))
	} else {
		return out, nil
	}
}

func printChoice(dataHandler HasUserChoise, isMultiChoice bool) {
	data := dataHandler.GetNamedData()

	//header
	fmt.Print(dataHandler.GetDescription())
	if isMultiChoice {
		fmt.Println(" (Multiple space separated numbers) (Type 0 to exit)")
	} else {
		fmt.Println(" (Single number) (Type 0 to exit)")
	}

	//max width of name
	max := 0
	for _, n := range data {
		if len(n.GetName()) > max {
			max = len(n.GetName())
		}
	}

	//calculate cols
	width, _ := getSize()
	cols := (width - 5) / (max + 7)
	part := "%5v. %-" + strconv.Itoa(max) + "v"

	//calculate rows
	rows := len(data) / cols
	if len(data) % cols > 0 {
		rows++
	}

	for y := 0; y < rows; y++ {
		vals := make([]interface{}, 0)
		format := ""
		for x := 0; x < cols; x++ {
			id := y * cols + x
			if id < len(data) {
				vals = append(vals, 0, "")
				format = format + part
				vals[x*2] = id + 1
				vals[x*2 + 1] = data[id].GetName()
			}
		}
		fmt.Printf(format + "\n", vals...)
	}

	fmt.Printf("%5v. %v\n", 0, "Exit")
}

func getSize() (width int, height int) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		log.Printf("Failed to get terminal size due to %v", err)
	}
	_, err = fmt.Fscanf(bytes.NewReader(out), "%d %d", &height, &width)
	if err != nil {
		log.Printf("Failed to get terminal size due to %v", err)
	}
	return
}
