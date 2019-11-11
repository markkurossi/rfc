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
	"sort"
	"strconv"
	"strings"
)

var (
	reRFC      = regexp.MustCompilePOSIX(`^([[:digit:]]+) (.+)\. (.*)`)
	reParams   = regexp.MustCompilePOSIX(`[[:space:]]*\(([^\)]+)\)(.*)`)
	reForward  = regexp.MustCompilePOSIX(`(Obsoleted|Updated) by (.*)`)
	reBackward = regexp.MustCompilePOSIX(`(Obsoletes|Updates) (.*)`)
	reStatus   = regexp.MustCompilePOSIX(`Status:[[:space:]]*(.*)`)
	reRef      = regexp.MustCompilePOSIX(`RFC([[:digit:]]+)(.*)`)

	RFCs      = make(map[string]*RFC)
	processed = make(map[string]*RFC)
	edgeMap   = make(map[string]Edge)
	graphs    []Graph
)

func GetRFC(id string) *RFC {
	rfc, ok := RFCs[id]
	if !ok {
		panic(fmt.Sprintf("Unknown RFC %s", id))
	}
	return rfc
}

type Type int

func (t Type) Edge() string {
	switch t {
	case Updated:
		return ""
	case Obsoleted:
		return " [style=dashed]"
	}
	panic(fmt.Sprintf("Unknown Type %s", t))
}

func (t Type) Node() string {
	switch t {
	case Current:
		return "style=solid fontname=\"Helvetica-Bold\""

	case Updated:
		return "style=solid fontname=\"Helvetica\""

	case Obsoleted:
		return "style=dotted fontname=\"Helvetica-Narrow\""
	}
	panic(fmt.Sprintf("Unknown type %s", t))
}

func MakeType(val string) Type {
	switch val {
	case "Updated", "Updates":
		return Updated
	case "Obsoleted", "Obsoletes":
		return Obsoleted
	default:
		panic(fmt.Sprintf("Invalid type %s", val))
	}
}

const (
	Current Type = iota
	Updated
	Obsoleted
)

type Edge struct {
	From string
	To   string
	Type Type
}

func (e Edge) ID() string {
	return fmt.Sprintf("%s:%s:%d", e.From, e.To, e.Type)
}

type Status int

var statusNode = map[Status]string{
	Unknown:             "none",
	Historic:            "cylinder",
	Experimental:        "parallelogram",
	Informational:       "house",
	DraftStandard:       "polygon",
	ProposedStandard:    "oval",
	InternetStandard:    "box",
	BestCurrentPractice: "trapezium",
}

var statusName = map[Status]string{
	Unknown:             "Unknown",
	Historic:            "Historic",
	Experimental:        "Experimental",
	Informational:       "Informational",
	DraftStandard:       "DraftStandard",
	ProposedStandard:    "ProposedStandard",
	InternetStandard:    "InternetStandard",
	BestCurrentPractice: "BestCurrentPractice",
}

func (s Status) Node() string {
	node, ok := statusNode[s]
	if ok {
		return fmt.Sprintf("shape=%s", node)
	}
	panic(fmt.Sprintf("Unknown status %v", s))
}

func (s Status) String() string {
	name, ok := statusName[s]
	if ok {
		return name
	}
	return fmt.Sprintf("Status%d", s)
}

const (
	Unknown Status = iota
	Historic
	Experimental
	Informational
	DraftStandard
	ProposedStandard
	InternetStandard
	BestCurrentPractice
)

var Statuses = map[string]Status{
	"UNKNOWN":               Unknown,
	"HISTORIC":              Historic,
	"EXPERIMENTAL":          Experimental,
	"INFORMATIONAL":         Informational,
	"DRAFT STANDARD":        DraftStandard,
	"PROPOSED STANDARD":     ProposedStandard,
	"INTERNET STANDARD":     InternetStandard,
	"BEST CURRENT PRACTICE": BestCurrentPractice,
}

func GetStatus(val string) Status {
	status, ok := Statuses[val]
	if ok {
		return status
	}
	panic(fmt.Sprintf("Unknown status %s", val))
}

type RFC struct {
	Number    string
	Title     string
	Authors   string
	Date      string
	Type      Type
	Status    Status
	Forwards  map[string]Type
	Backwards map[string]Type
	Params    []string
}

func (rfc *RFC) SetTypes() {
	for _, t := range rfc.Forwards {
		rfc.SetType(t)
	}
	for id, t := range rfc.Backwards {
		GetRFC(id).SetType(t)
	}
}

func (rfc *RFC) SetType(t Type) {
	if t > rfc.Type {
		rfc.Type = t
	}
}

func (rfc *RFC) Year() (int, error) {
	parts := strings.Split(rfc.Date, " ")
	if len(parts) != 2 {
		return 0, fmt.Errorf("Invalid RFC Date: %s", rfc.Date)
	}
	i, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	return i, nil
}

func (rfc *RFC) String() string {
	return fmt.Sprintf("%s;%s", rfc.Number, rfc.Title)
}

type Graph struct {
	Leader *RFC
	Size   int
}

func main() {
	index := flag.String("i", "rfc-index.txt", "RFC index file")
	traverse := flag.String("t", "", "RFC number to traverse")
	size := flag.Int("s", 0, "The minimum graph size")
	list := flag.Bool("l", false, "List graphs")
	graph := flag.Bool("g", false, "Print RFC graphs")
	root := flag.String("r", "", "Graph roots, default to all > size")
	timeline := flag.Bool("h", false, "Print timeline and legend")
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
					RFCs[rfc.Number] = rfc
				}
				break
			}
			line += " " + l
		}
	}

	// Set RFC types.
	for _, rfc := range RFCs {
		rfc.SetTypes()
	}

	var rfcs []*RFC
	if len(*root) > 0 {
		numbers := strings.Split(*root, ",")
		for _, num := range numbers {
			rfcs = append(rfcs, GetRFC(num))
		}
	} else {
		for _, rfc := range RFCs {
			rfcs = append(rfcs, rfc)
		}
	}

	// Find graphs larger than *size.
	findGraphs(rfcs, *size)

	// SSH: 4250
	// TLS: 4346
	if len(*traverse) > 0 {
		printTree(*traverse)
	}
	if *list {
		sort.SliceStable(graphs, func(i, j int) bool {
			return graphs[i].Leader.Number < graphs[j].Leader.Number
		})
		for _, g := range graphs {
			fmt.Printf("%s\t%d\t%s\n", g.Leader.Number, g.Size, g.Leader.Title)
		}
	}
	if *graph {
		printGraph(*timeline)
	}
}

func parseRFC(line string) *RFC {
	m := reRFC.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	parts := strings.Split(m[2], ". ")

	rfc := &RFC{
		Number:    m[1],
		Title:     parts[0],
		Authors:   strings.Join(parts[1:len(parts)-1], ". "),
		Date:      parts[len(parts)-1],
		Forwards:  make(map[string]Type),
		Backwards: make(map[string]Type),
	}

	params := m[3]
	for {
		m := reParams.FindStringSubmatch(params)
		if m == nil {
			break
		}
		mf := reForward.FindStringSubmatch(m[1])
		mb := reBackward.FindStringSubmatch(m[1])
		if mf != nil {
			for _, ref := range parseRefs(mf[2]) {
				rfc.Forwards[ref] = MakeType(mf[1])
			}
		} else if mb != nil {
			for _, ref := range parseRefs(mb[2]) {
				rfc.Backwards[ref] = MakeType(mb[1])
			}
		} else {
			ms := reStatus.FindStringSubmatch(m[1])
			if ms != nil {
				rfc.Status = GetStatus(ms[1])
			} else {
				rfc.Params = append(rfc.Params, m[1])
			}
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
	rfc := GetRFC(id)
	seen[id] = rfc
	fmt.Printf("%s\n", rfc)

	for r, _ := range rfc.Forwards {
		traverse(r, seen)
	}
	for r, _ := range rfc.Backwards {
		traverse(r, seen)
	}
}

func findGraphs(rfcs []*RFC, size int) {
	for _, id := range rfcs {
		count, leader := countGraph(id.Number, processed)

		if count >= size && count > 0 {
			graphs = append(graphs, Graph{
				Leader: leader,
				Size:   count,
			})
			collectEdges(leader.Number, processed, edgeMap)
		}
	}
}

func printGraph(timeline bool) {
	fmt.Printf("digraph rfc {\n")

	var nodes [Obsoleted + 1][BestCurrentPractice + 1][]*RFC
	var from, to int
	var ranks = make(map[int][]*RFC)

	for _, rfc := range RFCs {
		_, ok := processed[rfc.Number]
		if !ok {
			continue
		}
		year, err := rfc.Year()
		if err != nil {
			panic(err.Error())
		}
		if from == 0 || year < from {
			from = year
		}
		if to == 0 || year > to {
			to = year
		}
		ranks[year] = append(ranks[year], rfc)

		nodes[rfc.Type][rfc.Status] = append(nodes[rfc.Type][rfc.Status], rfc)
	}

	if timeline {
		fmt.Printf("// %d-%d\n", from, to)
		fmt.Printf("\tnode [shape=plaintext];\n\t%d", from)
		for i := from + 1; i <= to; i++ {
			fmt.Printf(" -> %d", i)
		}
		fmt.Printf(";\n\t%d -> Legend [style=invis];\n", to)
	}

	for t, tarr := range nodes {
		for s, arr := range tarr {
			fmt.Printf("\tnode [%s %s]\n", Status(s).Node(), Type(t).Node())
			for _, rfc := range arr {
				fmt.Printf("\t%s;\n", rfc.Number)
			}
		}
	}

	if timeline {
		for i := from; i <= to; i++ {
			arr, ok := ranks[i]
			if !ok {
				continue
			}
			fmt.Printf("\t{rank=same %d", i)
			for _, rfc := range arr {
				fmt.Printf(" %s", rfc.Number)
			}
			fmt.Printf("}\n")
		}

		for status, _ := range statusName {
			fmt.Printf("\tnode [%s style=solid];\n", status.Node())
			fmt.Printf("\t%s;\n", status)
		}
		fmt.Printf("\t{rank=same Legend")
		for status, _ := range statusName {
			fmt.Printf(" %s", status)
		}
		fmt.Printf("}\n")
	}

	var edges []Edge
	for _, edge := range edgeMap {
		edges = append(edges, edge)
	}

	sort.SliceStable(edges, func(i, j int) bool {
		if edges[i].From < edges[j].From {
			return true
		}
		if edges[i].From > edges[j].From {
			return false
		}
		return edges[i].To < edges[j].To
	})

	for _, edge := range edges {
		fmt.Printf("\t%s -> %s%s;\n", edge.From, edge.To, edge.Type.Edge())
	}

	fmt.Printf("}\n")
}

func collectEdges(id string, processed map[string]*RFC, edges map[string]Edge) {
	_, ok := processed[id]
	if ok {
		return
	}
	rfc := GetRFC(id)
	processed[id] = rfc

	for r, t := range rfc.Forwards {
		edge := Edge{
			From: id,
			To:   r,
			Type: t,
		}
		edges[edge.ID()] = edge
		collectEdges(r, processed, edges)
	}
	for r, t := range rfc.Backwards {
		edge := Edge{
			From: r,
			To:   id,
			Type: t,
		}
		edges[edge.ID()] = edge
		collectEdges(r, processed, edges)
	}
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
	rfc := GetRFC(id)
	graph[id] = rfc

	var c = 1
	for r, _ := range rfc.Forwards {
		c += count(r, graph, processed)
	}
	for r, _ := range rfc.Backwards {
		c += count(r, graph, processed)
	}
	return c
}
