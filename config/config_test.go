package config

import (
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_Config(t *testing.T) {
	Load()
	for k, v := range defaults {
		assert.Equal(t, v, viper.Get(k))
	}
}
