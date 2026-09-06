package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCreatesUserWithoutLocalCredentials(t *testing.T) {
	user, err := New(" Ada Lovelace ", " ADA@EXAMPLE.COM ")

	require.NoError(t, err)
	assert.Equal(t, "Ada Lovelace", user.Name)
	assert.Equal(t, "ada@example.com", user.Email)
	assert.NotEmpty(t, user.ID)
	assert.False(t, user.CreatedAt.IsZero())
	assert.Equal(t, user.CreatedAt, user.UpdatedAt)
}

func TestNewValidatesProfile(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  error
	}{
		{name: "A", email: "ada@example.com", want: ErrInvalidName},
		{name: "Ada", email: "not-an-email", want: ErrInvalidEmail},
	}

	for _, tt := range tests {
		_, err := New(tt.name, tt.email)
		assert.ErrorIs(t, err, tt.want)
	}
}
