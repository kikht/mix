package mix

type Player interface {
	Play(source Source)
}

type SourceMutator func(cur Source, pos Tz) Source

type SwitchPlayer interface {
	Switch(mutator SourceMutator)
}
