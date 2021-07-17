package pushstate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tkandal/checksum"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

/*
 * Copyright (c) 2019 Norwegian University of Science and Technology
 */

// FileCache hold check-sums and persists them to a file
type FileCache struct {
	filename   string
	checkSum   checksum.CheckSum
	log        *zap.SugaredLogger
	stateCache map[string]string
	isDirty    bool
	// Protect this cache
	cacheLock *sync.Mutex
}

func NewFileCache(sf string, cs checksum.CheckSum, log *zap.SugaredLogger) *FileCache {
	return &FileCache{
		filename:   sf,
		checkSum:   cs,
		log:        log,
		stateCache: map[string]string{},
		isDirty:    false,
		cacheLock:  &sync.Mutex{},
	}
}

func (fc *FileCache) getCache() map[string]string {
	if fc.stateCache == nil {
		fc.cacheLock = &sync.Mutex{}
	}
	if fc.stateCache == nil {
		fc.stateCache = map[string]string{}
	}
	return fc.stateCache
}

// IsChanged checks if the card is new or changed
func (fc *FileCache) IsChanged(m PushModel) bool {
	fc.cacheLock.Lock()
	defer fc.cacheLock.Unlock()

	if len(fc.getCache()[m.GetId()]) == 0 {
		return true
	}

	return fc.getCache()[m.GetId()] != fc.makeCheckSum(m)
}

// Put puts the card's check-sum in the cache
func (fc *FileCache) Put(m PushModel) {
	fc.cacheLock.Lock()
	defer fc.cacheLock.Unlock()

	fc.getCache()[m.GetId()] = fc.makeCheckSum(m)
	fc.isDirty = true
}

func readFile(filename string) (map[string]string, error) {
	stateFile, err := os.OpenFile(filename, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open %s failed; error = %v", filename, err)
	}
	defer func() {
		_ = stateFile.Close()
	}()

	cache := map[string]string{}
	if err = json.NewDecoder(stateFile).Decode(&cache); err != nil && err != io.EOF {
		return nil, fmt.Errorf("decode state-file %s failed; error = %v", filename, err)
	}
	return cache, nil
}

func (fc *FileCache) Read() error {
	fc.cacheLock.Lock()
	defer fc.cacheLock.Unlock()

	cache, err := readFile(fc.filename)
	if err != nil {
		return err
	}
	fc.stateCache = cache
	return nil
}

func (fc *FileCache) saveToFile(filename string, cache map[string]string) error {
	tmpFile, err := ioutil.TempFile(filepath.Dir(filename), filepath.Base(filename))
	if err != nil {
		return fmt.Errorf("create temporary file failed; error = %v", err)
	}

	if err = json.NewEncoder(tmpFile).Encode(cache); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return fmt.Errorf("encode to %s failed; error = %v", tmpFile.Name(), err)
	}
	if err = tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFile.Name())
		return fmt.Errorf("close %s failed; error = %v", tmpFile.Name(), err)
	}
	if err = os.Rename(tmpFile.Name(), filename); err != nil {
		return fmt.Errorf("rename %s to %s failed; error = %v", tmpFile.Name(), filename, err)
	}

	if err  = os.Chmod(filename, os.FileMode(0640)); err != nil {
		fc.log.Warnw(fmt.Sprintf("chmod on %s failed", filename), "error", err)
	}
	fc.log.Debugf("saved state-cache to %s", filename)

	return nil
}

// Save saves the check-sums to a file
func (fc *FileCache) Save() error {
	if !fc.isDirty {
		return nil
	}
	fc.cacheLock.Lock()
	defer fc.cacheLock.Unlock()

	if err := fc.saveToFile(fc.filename, fc.getCache()); err != nil {
		return err
	}
	fc.isDirty = false
	return nil
}

// Size returns the number of check-sums
func (fc *FileCache) Size() int64 {
	fc.cacheLock.Lock()
	defer fc.cacheLock.Unlock()
	return int64(len(fc.getCache()))
}

// Get returns the check-sum for the given id
func (fc *FileCache) Get(id string) string {
	fc.cacheLock.Lock()
	defer fc.cacheLock.Unlock()

	return fc.getCache()[id]
}

// Delete deletes the check-sum for the given id
func (fc *FileCache) Delete(id string) error {
	fc.cacheLock.Lock()
	defer fc.cacheLock.Unlock()

	delete(fc.getCache(), id)
	fc.isDirty = true
	if err := fc.saveToFile(fc.filename, fc.getCache()); err != nil {
		return err
	}
	fc.isDirty = false
	return nil
}

// Reset empties the cache
func (fc *FileCache) Reset() error {
	fc.cacheLock.Lock()
	defer fc.cacheLock.Unlock()

	cache := map[string]string{}
	fc.isDirty = true
	if err := fc.saveToFile(fc.filename, cache); err != nil {
		return err
	}
	fc.stateCache = cache
	fc.isDirty = false
	return nil
}

// Dump dumps the whole content to an io.Reader
func (fc *FileCache) Dump() (io.Reader, error) {
	fc.cacheLock.Lock()
	defer fc.cacheLock.Unlock()

	stateFile, err := os.OpenFile(fc.filename, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open %s failed; error = %v", fc.filename, err)
	}
	defer func() {
		if err := stateFile.Close(); err != nil {
			fc.log.Warnw(fmt.Sprintf("close %s failed", fc.filename), "error", err)
		}
	}()

	stats, err := stateFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s failed; error = %v", fc.filename, err)
	}
	buf := bytes.NewBuffer(make([]byte, stats.Size()))
	buf.Reset()
	_, err = io.Copy(buf, stateFile)
	if err != nil {
		return nil, fmt.Errorf("copy %s to buffer failed; error = %v", fc.filename, err)
	}
	return buf, nil
}

func (fc *FileCache) WriteTo(w io.Writer) (int64, error) {
	fc.cacheLock.Lock()
	defer fc.cacheLock.Unlock()

	stateFile, err := os.OpenFile(fc.filename, os.O_CREATE|os.O_RDONLY, os.FileMode(0644))
	if err != nil {
		return 0, fmt.Errorf("open %s failed; error = %v", fc.filename, err)
	}
	defer func() {
		if err := stateFile.Close(); err != nil {
			fc.log.Warnf("close %s failed; error = %v", fc.filename, err)
		}
	}()
	return io.Copy(w, stateFile)
}

func (fc *FileCache) makeCheckSum(v interface{}) string {
	jsonBuf := &bytes.Buffer{}
	if err := json.NewEncoder(jsonBuf).Encode(v); err != nil {
		return ""
	}
	return fc.checkSum.SumBytes(jsonBuf.Bytes())
}
