package pushstate

import (
	"git.it.ntnu.no/df/tia/frontend/cards2bas/model"
	"io"
)

/*
 * Copyright (c) 2019 Norwegian University of Science and Technology
 */

// Cacher holds check-sums, check if a struct is new/changed, restores check-sums and saves check-sums to persistent storage
type Cacher interface {
	IsChanged(model.PushModel) bool
	Put(model.PushModel)
	Read() error
	Save() error
	Size() int64
	Get(string) string
	Delete(string) error
	Reset() error
	Dump() (io.Reader, error)
}
