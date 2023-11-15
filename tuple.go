package zima

type Tuple struct {
	Parent Set
	Child  Set
}

func NewTuple(parent Set, child Set) Tuple {
	return Tuple{parent, child}
}
