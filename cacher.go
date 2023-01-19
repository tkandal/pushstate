package pushstate

import (
	"io"
)

/*
 * Copyright (c) 2019 Norwegian University of Science and Technology
 */

type PushModel interface {
	GetID() string
}

// Cacher holds check-sums, check if a struct is new/changed, restores check-sums and saves check-sums to persistent storage
type Cacher interface {
	IsChanged(PushModel) bool
	Put(PushModel)
	Read() error
	Save() error
	Size() int64
	Get(string) string
	Delete(string) error
	Reset() error
	Dump() (io.Reader, error)
	WriteTo(io.Writer) (int64, error)
}
