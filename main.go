//
// main.go
//
// Copyright (c) 2019 Markku Rossi
//
// All rights reserved.
//

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	reRFC      = regexp.MustCompilePOSIX(`^([[:digit:]]+) (.+)\. (.*)`)
	reParams   = regexp.MustCompilePOSIX(`[[:space:]]*\(([^\)]+)\)(.*)`)
	reForward  = regexp.MustCompilePOSIX(`(Obsoleted|Updated) by (.*)`)
	reBackward = regexp.MustCompilePOSIX(`(Obsoletes|Updates) (.*)`)
	reRef      = regexp.MustCompilePOSIX(`RFC([[:digit:]]+)(.*)`)

	RFCs = make(map[string]*RFC)
)

type Type int

const (
	Updated Type = iota
	Obsoleted
)

type RFC struct {
	Number    string
	Title     string
	Authors   string
	Date      string
	Forwards  []string
	Backwards []string
	Params    []string
}

func (rfc *RFC) String() string {
	return fmt.Sprintf("%s;%s", rfc.Number, rfc.Title)
}

func main() {
	index := flag.String("i", "rfc-index.txt", "RFC index file")
	traverse := flag.String("t", "", "RFC number to traverse")
	graph := flag.Int("g", 0, "Print RFC graphs with size >= limit")
	flag.Parse()

	file, err := os.Open(*index)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}
		for scanner.Scan() {
			l := strings.TrimSpace(scanner.Text())
			if len(l) == 0 {
				rfc := parseRFC(line)
				if rfc != nil {
					// fmt.Printf("%s\n", rfc)
					RFCs[rfc.Number] = rfc
				}
				break
			}
			line += " " + l
		}
	}

	// SSH: 4250
	// TLS: 4346
	if len(*traverse) > 0 {
		printTree(*traverse)
	}
	if *graph > 0 {
		printGraph(*graph)
	}
}

func parseRFC(line string) *RFC {
	m := reRFC.FindStringSubmatch(line)
	if m == nil {
		// fmt.Printf("Invalid: %s\n", line)
		return nil
	}
	parts := strings.Split(m[2], ". ")

	rfc := &RFC{
		Number:  m[1],
		Title:   parts[0],
		Authors: strings.Join(parts[1:len(parts)-1], ". "),
		Date:    parts[len(parts)-1],
	}

	// fmt.Printf("%s %s\n - %s\n", m[1], m[2], m[3])

	params := m[3]
	for {
		m := reParams.FindStringSubmatch(params)
		if m == nil {
			break
		}
		mf := reForward.FindStringSubmatch(m[1])
		mb := reBackward.FindStringSubmatch(m[1])
		if mf != nil {
			rfc.Forwards = append(rfc.Forwards, parseRefs(mf[2])...)
			// fmt.Printf(" -> %s\n", strings.Join(parseRefs(mf[2]), " "))
		} else if mb != nil {
			rfc.Backwards = append(rfc.Backwards, parseRefs(mb[2])...)
			// fmt.Printf(" <- %s\n", strings.Join(parseRefs(mb[2]), " "))
		} else {
			rfc.Params = append(rfc.Params, m[1])
			// fmt.Printf(" -  %s\n", m[1])
		}

		params = m[2]
	}
	return rfc
}

func parseRefs(input string) []string {
	var result []string

	for {
		m := reRef.FindStringSubmatch(input)
		if m == nil {
			return result
		}
		result = append(result, m[1])
		input = m[2]
	}
}

func printTree(root string) {
	seen := make(map[string]*RFC)

	traverse(root, seen)
}

func traverse(id string, seen map[string]*RFC) {
	_, ok := seen[id]
	if ok {
		return
	}
	rfc, ok := RFCs[id]
	if !ok {
		for k, _ := range RFCs {
			fmt.Printf("Key: %s\n", k)
		}
		panic(fmt.Sprintf("Unknown RFC %s", id))
	}
	seen[id] = rfc
	fmt.Printf("%s\n", rfc)

	for _, r := range rfc.Forwards {
		traverse(r, seen)
	}
	for _, r := range rfc.Backwards {
		traverse(r, seen)
	}
}

func printGraph(size int) {
	processed := make(map[string]*RFC)

	fmt.Printf("digraph rfc {\n")

	for id, _ := range RFCs {
		count, leader := countGraph(id, processed)

		if count >= size {
			fmt.Printf("// Graph %s\t%d\t%s\n",
				leader.Number, count, leader.Title)
			print(id, processed)
		}
	}
	fmt.Printf("}\n")
}

func print(id string, processed map[string]*RFC) bool {
	_, ok := processed[id]
	if ok {
		return false
	}
	rfc, ok := RFCs[id]
	if !ok {
		panic(fmt.Sprintf("Unknown RFC %s", id))
	}
	processed[id] = rfc

	for _, r := range rfc.Forwards {
		if print(r, processed) {
			fmt.Printf("\t%s -> %s;\n", id, r)
		}
	}
	for _, r := range rfc.Backwards {
		if print(r, processed) {
			fmt.Printf("\t%s -> %s;\n", id, r)
		}
	}
	return true
}

func countGraph(id string, processed map[string]*RFC) (cnt int, leader *RFC) {
	graph := make(map[string]*RFC)
	cnt = count(id, graph, processed)
	if cnt > 0 {
		for _, rfc := range graph {
			if leader == nil || leader.Number > rfc.Number {
				leader = rfc
			}
		}
	}
	return
}

func count(id string, graph, processed map[string]*RFC) int {
	_, ok := processed[id]
	if ok {
		return 0
	}
	_, ok = graph[id]
	if ok {
		return 0
	}
	rfc, ok := RFCs[id]
	if !ok {
		panic(fmt.Sprintf("Unknown RFC %s", id))
	}
	graph[id] = rfc

	var c = 1
	for _, r := range rfc.Forwards {
		c += count(r, graph, processed)
	}
	for _, r := range rfc.Backwards {
		c += count(r, graph, processed)
	}
	return c
}
