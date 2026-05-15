package callgraph

type Greeter interface {
	Greet(name string)
}

type EnglishGreeter struct{}

func (g *EnglishGreeter) Greet(name string) {}
