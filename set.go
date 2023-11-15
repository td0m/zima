package zima

type Set struct {
	Type     string
	ID       string
	Relation string
}

func (s Set) IsSingleton() bool {
	return s.Relation == ""
}

func NewSet(typ, id, relation string) (Set, error) {
	return Set{typ, id, relation}, nil
}
