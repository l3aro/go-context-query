package dirty

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracker_New(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{
			name: "default options",
			opts: nil,
		},
		{
			name: "custom cache dir",
			opts: []Option{WithCacheDir(".test-cache")},
		},
		{
			name: "custom cache file",
			opts: []Option{WithCacheFile("test-dirty.json")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var tracker *Tracker
			if tc.opts == nil {
				tracker = New()
			} else {
				tracker = New(tc.opts...)
			}

			assert.NotNil(t, tracker)
			assert.Equal(t, 0, tracker.Count())
		})
	}
}

func TestTracker_MarkDirty(t *testing.T) {
	// Create a temp file to mark dirty
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte("package test\nfunc main() {}"), 0644)
	require.NoError(t, err)

	tracker := New()

	err = tracker.MarkDirty(testFile)
	require.NoError(t, err)
	assert.Equal(t, 1, tracker.Count())
	assert.True(t, tracker.IsDirty(testFile))

	// Marking same file again should not increase count
	err = tracker.MarkDirty(testFile)
	require.NoError(t, err)
	assert.Equal(t, 1, tracker.Count())
}

func TestTracker_MarkDirty_NonExistent(t *testing.T) {
	tracker := New()
	err := tracker.MarkDirty("/non/existent/file.go")
	assert.Error(t, err)
}

func TestTracker_IsDirty(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte("package test"), 0644)
	require.NoError(t, err)

	tracker := New()

	// File not tracked yet
	assert.False(t, tracker.IsDirty(testFile))

	// Mark dirty
	err = tracker.MarkDirty(testFile)
	require.NoError(t, err)
	assert.True(t, tracker.IsDirty(testFile))

	// Clear dirty
	tracker.ClearDirty([]string{testFile})
	assert.False(t, tracker.IsDirty(testFile))
}

func TestTracker_GetDirtyFiles(t *testing.T) {
	tmpDir := t.TempDir()

	files := make([]string, 3)
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(tmpDir, "test"+string(rune('a'+i))+".go")
		err := os.WriteFile(testFile, []byte("package test"), 0644)
		require.NoError(t, err)
		files[i] = testFile
	}

	tracker := New()

	// No dirty files initially
	assert.Empty(t, tracker.GetDirtyFiles())

	// Mark some files dirty
	err := tracker.MarkDirty(files[0])
	require.NoError(t, err)
	err = tracker.MarkDirty(files[2])
	require.NoError(t, err)

	dirtyFiles := tracker.GetDirtyFiles()
	assert.Len(t, dirtyFiles, 2)
	assert.Contains(t, dirtyFiles, files[0])
	assert.Contains(t, dirtyFiles, files[2])
}

func TestTracker_ClearDirty(t *testing.T) {
	tmpDir := t.TempDir()

	files := make([]string, 3)
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(tmpDir, "test"+string(rune('a'+i))+".go")
		err := os.WriteFile(testFile, []byte("package test"), 0644)
		require.NoError(t, err)
		files[i] = testFile
	}

	tracker := New()

	// Mark all files dirty
	for _, f := range files {
		err := tracker.MarkDirty(f)
		require.NoError(t, err)
	}
	assert.Equal(t, 3, tracker.Count())

	// Clear specific files
	tracker.ClearDirty(files[:2])
	assert.Equal(t, 1, tracker.Count())
	assert.True(t, tracker.IsDirty(files[2]))
	assert.False(t, tracker.IsDirty(files[0]))
	assert.False(t, tracker.IsDirty(files[1]))

	// Clear all remaining
	tracker.ClearDirty(nil)
	assert.Equal(t, 0, tracker.Count())
}

func TestTracker_Count(t *testing.T) {
	tmpDir := t.TempDir()

	tracker := New()
	assert.Equal(t, 0, tracker.Count())

	// Add files
	for i := 0; i < 5; i++ {
		testFile := filepath.Join(tmpDir, "test"+string(rune('a'+i))+".go")
		err := os.WriteFile(testFile, []byte("package test"), 0644)
		require.NoError(t, err)
		err = tracker.MarkDirty(testFile)
		require.NoError(t, err)
	}

	assert.Equal(t, 5, tracker.Count())

	// Clear some
	tracker.ClearDirty([]string{})
	assert.Equal(t, 0, tracker.Count())
}

func TestTracker_CheckAndMark(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte("package test"), 0644)
	require.NoError(t, err)

	tracker := New()

	// First check - should mark dirty
	changed, err := tracker.CheckAndMark(testFile)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, tracker.IsDirty(testFile))

	// Second check - same content - should not mark dirty
	changed, err = tracker.CheckAndMark(testFile)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.False(t, tracker.IsDirty(testFile))

	// Modify file
	err = os.WriteFile(testFile, []byte("package test\nfunc main() {}"), 0644)
	require.NoError(t, err)

	// Third check - content changed - should mark dirty
	changed, err = tracker.CheckAndMark(testFile)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, tracker.IsDirty(testFile))
}

func TestTracker_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()

	files := make([]string, 3)
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(tmpDir, "test"+string(rune('a'+i))+".go")
		err := os.WriteFile(testFile, []byte("package test"), 0644)
		require.NoError(t, err)
		files[i] = testFile
	}

	// Create tracker and mark some files dirty
	tracker := New(WithCacheDir(tmpDir))
	for _, f := range files[:2] {
		err := tracker.MarkDirty(f)
		require.NoError(t, err)
	}

	// Save
	err := tracker.Save()
	require.NoError(t, err)

	// Create new tracker and load
	tracker2 := New(WithCacheDir(tmpDir))
	err = tracker2.Load()
	require.NoError(t, err)

	assert.Equal(t, 2, tracker2.Count())
	assert.True(t, tracker2.IsDirty(files[0]))
	assert.True(t, tracker2.IsDirty(files[1]))
	assert.False(t, tracker2.IsDirty(files[2]))
}

func TestTracker_SaveToLoadFrom(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte("package test"), 0644)
	require.NoError(t, err)

	tracker := New()
	err = tracker.MarkDirty(testFile)
	require.NoError(t, err)

	// Save to buffer
	var buf bytes.Buffer
	err = tracker.SaveTo(&buf)
	require.NoError(t, err)

	// Load from buffer
	tracker2 := New()
	err = tracker2.LoadFrom(&buf)
	require.NoError(t, err)

	assert.Equal(t, 1, tracker2.Count())
	assert.True(t, tracker2.IsDirty(testFile))
}

func TestTracker_Remove(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte("package test"), 0644)
	require.NoError(t, err)

	tracker := New()
	err = tracker.MarkDirty(testFile)
	require.NoError(t, err)
	assert.Equal(t, 1, tracker.Count())

	tracker.Remove(testFile)
	assert.Equal(t, 0, tracker.Count())
	assert.False(t, tracker.IsDirty(testFile))
}

func TestTracker_Clear(t *testing.T) {
	tmpDir := t.TempDir()

	files := make([]string, 3)
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(tmpDir, "test"+string(rune('a'+i))+".go")
		err := os.WriteFile(testFile, []byte("package test"), 0644)
		require.NoError(t, err)
		files[i] = testFile
	}

	tracker := New(WithCacheDir(tmpDir))
	for _, f := range files {
		err := tracker.MarkDirty(f)
		require.NoError(t, err)
	}
	assert.Equal(t, 3, tracker.Count())

	tracker.Clear()
	assert.Equal(t, 0, tracker.Count())
}

func TestTracker_GetHash(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	content := []byte("package test")
	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	tracker := New()

	// Not tracked yet
	hash, exists := tracker.GetHash(testFile)
	assert.Empty(t, hash)
	assert.False(t, exists)

	// Mark dirty
	err = tracker.MarkDirty(testFile)
	require.NoError(t, err)

	// Now should have hash
	hash, exists = tracker.GetHash(testFile)
	assert.NotEmpty(t, hash)
	assert.True(t, exists)
}

func TestTracker_TotalCount(t *testing.T) {
	tmpDir := t.TempDir()

	tracker := New()
	assert.Equal(t, 0, tracker.TotalCount())

	for i := 0; i < 3; i++ {
		testFile := filepath.Join(tmpDir, "test"+string(rune('a'+i))+".go")
		err := os.WriteFile(testFile, []byte("package test"), 0644)
		require.NoError(t, err)
		err = tracker.MarkDirty(testFile)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, tracker.TotalCount())

	// Clear dirty but keep tracked
	tracker.ClearDirty(nil)
	assert.Equal(t, 0, tracker.Count())
	assert.Equal(t, 3, tracker.TotalCount())
}
