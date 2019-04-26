package mix

type PlayerState interface {
	ChunkSize() Tz
}

type Player interface {
	PlayerState
	Play(source Source)
}

type SwitchPlayer interface {
	PlayerState
	Switch(mutator SourceMutator)
}

type SourceMutator interface {
	Mutate(cur Source, pos Tz) Source
}

type SourceMutatorFunc func(cur Source, pos Tz) Source

func (f SourceMutatorFunc) Mutate(cur Source, pos Tz) Source {
	return f(cur, pos)
}
