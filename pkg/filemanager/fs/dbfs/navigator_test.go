package dbfs

import (
	"testing"

	"github.com/cloudreve/Cloudreve/v4/ent"
	"github.com/cloudreve/Cloudreve/v4/inventory"
	"github.com/cloudreve/Cloudreve/v4/inventory/types"
	"github.com/stretchr/testify/assert"
)

func TestApplyNaturalSort(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		order    inventory.OrderDirection
		expected []string
	}{
		{
			name:     "Natural sort ascending",
			input:    []string{"10.png", "2.png", "1.png", "20.png", "3.png", "11.png"},
			order:    inventory.OrderDirectionAsc,
			expected: []string{"1.png", "2.png", "3.png", "10.png", "11.png", "20.png"},
		},
		{
			name:     "Natural sort descending",
			input:    []string{"10.png", "2.png", "1.png", "20.png", "3.png", "11.png"},
			order:    inventory.OrderDirectionDesc,
			expected: []string{"20.png", "11.png", "10.png", "3.png", "2.png", "1.png"},
		},
		{
			name:     "Mixed files and folders ascending",
			input:    []string{"folder10", "file2.txt", "folder1", "file20.txt", "folder2", "file1.txt"},
			order:    inventory.OrderDirectionAsc,
			expected: []string{"folder1", "folder2", "folder10", "file1.txt", "file2.txt", "file20.txt"},
		},
		{
			name:     "Files with complex names",
			input:    []string{"Chapter 11", "Chapter 2", "Chapter 1", "Chapter 20"},
			order:    inventory.OrderDirectionAsc,
			expected: []string{"Chapter 1", "Chapter 2", "Chapter 11", "Chapter 20"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock files
			files := make([]*File, len(tt.input))
			for i, name := range tt.input {
				fileType := int(types.FileTypeFile)
				// Check if it's a folder based on the name pattern
				if len(name) >= 6 && name[:6] == "folder" {
					fileType = int(types.FileTypeFolder)
				} else if len(name) >= 7 && name[:7] == "Chapter" {
					fileType = int(types.FileTypeFolder)
				}
				files[i] = &File{
					Model: &ent.File{
						Name: name,
						Type: fileType,
					},
				}
			}

			// Apply natural sort
			applyNaturalSort(files, tt.order)

			// Check results
			for i, f := range files {
				assert.Equal(t, tt.expected[i], f.Name(), "File at position %d", i)
			}
		})
	}
}
