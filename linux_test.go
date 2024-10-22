//go:build linux
// +build linux

package lumberjack

import (
	"io/fs"
	"os"
	"syscall"
	"testing"
	"time"
)

func testMaintainMode(t *testing.T, fileMode fs.FileMode) {
	currentTime = fakeTime
	dir := t.TempDir()
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	l := &Logger{
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
		FileMode:   fileMode,
	}
	defer l.Close()

	mode := defaultFileMode
	// If custom file mode is set then use it.
	if l.fileModeIsSet() {
		mode = l.FileMode
	}

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode)
	isNil(err, t)
	f.Close()

	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	newFakeTime()

	err = l.Rotate()
	isNil(err, t)

	filename2 := backupFile(dir)
	info, err := os.Stat(filename)
	isNil(err, t)
	info2, err := os.Stat(filename2)
	isNil(err, t)
	equals(mode, info.Mode(), t)
	equals(mode, info2.Mode(), t)
}

func TestMaintainModeRestricted(t *testing.T) {
	testMaintainMode(t, os.FileMode(0600))
}

func TestMaintainModeCustom(t *testing.T) {
	testMaintainMode(t, os.FileMode(0644))
}

// This file mode is invalid not set, test should fallback
// to default file mode.
func TestMaintainModeEmpty(t *testing.T) {
	testMaintainMode(t, os.FileMode(0000))
}

// This file mode is invalid not set, test should fallback
// to default file mode.
func TestMaintainModeZero(t *testing.T) {
	testMaintainMode(t, 0)
}

func TestMaintainOwner(t *testing.T) {
	fakeFS := newFakeFS()
	osChown = fakeFS.Chown
	osStat = fakeFS.Stat
	defer func() {
		osChown = os.Chown
		osStat = os.Stat
	}()
	currentTime = fakeTime
	dir := makeTempDir("TestMaintainOwner", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, defaultFileMode)
	isNil(err, t)
	f.Close()

	l := &Logger{
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	newFakeTime()

	err = l.Rotate()
	isNil(err, t)

	equals(555, fakeFS.files[filename].uid, t)
	equals(666, fakeFS.files[filename].gid, t)
}

func testCompressMaintainMode(t *testing.T, fileMode fs.FileMode) {
	currentTime = fakeTime

	dir := t.TempDir()
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	l := &Logger{
		Compress:   true,
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
		FileMode:   fileMode,
	}
	defer l.Close()

	mode := defaultFileMode
	// If custom file mode is set then use it.
	if l.fileModeIsSet() {
		mode = l.FileMode
	}

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode)
	isNil(err, t)
	f.Close()

	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	newFakeTime()

	err = l.Rotate()
	isNil(err, t)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// a compressed version of the log file should now exist with the correct
	// mode.
	filename2 := backupFile(dir)
	info, err := os.Stat(filename)
	isNil(err, t)
	info2, err := os.Stat(filename2 + compressSuffix)
	isNil(err, t)
	equals(mode, info.Mode(), t)
	equals(mode, info2.Mode(), t)
}

func TestCompressMaintainModeRestricted(t *testing.T) {
	testCompressMaintainMode(t, os.FileMode(0600))
}

func TestCompressMaintainModeCustom(t *testing.T) {
	testCompressMaintainMode(t, os.FileMode(0644))
}

// This file mode is invalid not set, test should fallback
// to default file mode.
func TestCompressMaintainModeEmpty(t *testing.T) {
	testCompressMaintainMode(t, os.FileMode(0000))
}

// This file mode is invalid not set, test should fallback
// to default file mode.
func TestCompressMaintainModeZero(t *testing.T) {
	testCompressMaintainMode(t, 0)
}

func TestCompressMaintainOwner(t *testing.T) {
	fakeFS := newFakeFS()
	osChown = fakeFS.Chown
	osStat = fakeFS.Stat
	defer func() {
		osChown = os.Chown
		osStat = os.Stat
	}()
	currentTime = fakeTime
	dir := makeTempDir("TestCompressMaintainOwner", t)
	defer os.RemoveAll(dir)

	filename := logFile(dir)

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, defaultFileMode)
	isNil(err, t)
	f.Close()

	l := &Logger{
		Compress:   true,
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	newFakeTime()

	err = l.Rotate()
	isNil(err, t)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// a compressed version of the log file should now exist with the correct
	// owner.
	filename2 := backupFile(dir)
	equals(555, fakeFS.files[filename2+compressSuffix+tmpSuffix].uid, t)
	equals(666, fakeFS.files[filename2+compressSuffix+tmpSuffix].gid, t)
}

type fakeFile struct {
	uid int
	gid int
}

type fakeFS struct {
	files map[string]fakeFile
}

func newFakeFS() *fakeFS {
	return &fakeFS{files: make(map[string]fakeFile)}
}

func (fs *fakeFS) Chown(name string, uid, gid int) error {
	fs.files[name] = fakeFile{uid: uid, gid: gid}
	return nil
}

func (fs *fakeFS) Stat(name string) (os.FileInfo, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	stat.Uid = 555
	stat.Gid = 666
	return info, nil
}

func TestFileModeIsSet(t *testing.T) {
	type testCase struct {
		fileMode fs.FileMode
		isSet    bool
	}

	testCases := []testCase{
		{
			fileMode: os.FileMode(0600),
			isSet:    true,
		},
		{
			fileMode: os.FileMode(0644),
			isSet:    true,
		},
		{
			fileMode: os.FileMode(0000),
			isSet:    false,
		},
		{
			fileMode: 0,
			isSet:    false,
		},
	}

	for _, c := range testCases {
		l := &Logger{
			FileMode: c.fileMode,
		}
		equals(c.isSet, l.fileModeIsSet(), t)
	}
}

func testChangeModeOnRotate(t *testing.T, oldMode fs.FileMode, newMode fs.FileMode) {
	currentTime = fakeTime
	dir := t.TempDir()
	defer os.RemoveAll(dir)

	// create logger
	filename := logFile(dir)
	l := &Logger{
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
		FileMode:   oldMode,
	}
	defer l.Close()

	// create file
	mode := defaultFileMode
	if l.fileModeIsSet() {
		mode = l.FileMode
	}
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode)
	isNil(err, t)
	f.Close()

	// change logger file mode
	l.FileMode = newMode

	// write something to the file
	b := []byte("boo!")
	n, err := l.Write(b)
	isNil(err, t)
	equals(len(b), n, t)

	// check that file permissions didn't change
	info, err := os.Stat(filename)
	isNil(err, t)
	equals(mode, info.Mode(), t)

	// rotate the file
	newFakeTime()
	err = l.Rotate()
	isNil(err, t)

	// check permissions for the rotated file
	rotatedFilename := backupFile(dir)
	rotatedInfo, err := os.Stat(rotatedFilename)
	isNil(err, t)
	equals(mode, rotatedInfo.Mode(), t)
	// check permissions for the new file
	expected := oldMode
	if l.fileModeIsSet() {
		expected = l.FileMode
	}
	info, err = os.Stat(filename)
	isNil(err, t)
	equals(expected, info.Mode(), t)
}

func TestChangeModeOnRotate(t *testing.T) {
	testChangeModeOnRotate(t, 0600, 0644)
	testChangeModeOnRotate(t, 0644, 0600)
	testChangeModeOnRotate(t, 0000, 0640)
	testChangeModeOnRotate(t, 0640, 0000)
}
