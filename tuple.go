package zima

type Tuple struct {
	Parent Set `json:"parent"`
	Child  Set `json:"child"`
}

func NewTuple(parent Set, child Set) Tuple {
	return Tuple{parent, child}
}
