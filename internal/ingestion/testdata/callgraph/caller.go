package callgraph

import "fmt"

func Caller() {
	Callee()
	fmt.Println("done")
	g := &EnglishGreeter{}
	g.Greet("world")
}
