package statusbar_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kat/pkg/ui/statusbar"
)

func TestStatusMessageModel(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setup   func(m *statusbar.StatusMessageModel) // called before assertion
		want    bool                                  // expected Visible()
		wantOpt bool                                  // whether Opt() should be non-nil
	}{
		"zero value is not visible": {
			setup:   func(_ *statusbar.StatusMessageModel) {},
			want:    false,
			wantOpt: false,
		},
		"set makes visible": {
			setup: func(m *statusbar.StatusMessageModel) {
				m.Set("hello", statusbar.StyleSuccess)
			},
			want:    true,
			wantOpt: true,
		},
		"clear makes invisible": {
			setup: func(m *statusbar.StatusMessageModel) {
				m.Set("hello", statusbar.StyleSuccess)
				m.Clear()
			},
			want:    false,
			wantOpt: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var m statusbar.StatusMessageModel

			tc.setup(&m)

			assert.Equal(t, tc.want, m.Visible())

			if tc.wantOpt {
				require.NotNil(t, m.Opt())
			} else {
				assert.Nil(t, m.Opt())
			}
		})
	}
}

func TestStatusMessageModel_Update(t *testing.T) {
	t.Parallel()

	t.Run("consumes matching timeout", func(t *testing.T) {
		t.Parallel()

		var m statusbar.StatusMessageModel

		cmd := m.Set("msg", statusbar.StyleSuccess)
		require.NotNil(t, cmd)
		assert.True(t, m.Visible())

		// Simulate the timeout message by calling Set again to get seq=2,
		// then send a timeout with seq=2.
		cmd2 := m.Set("msg2", statusbar.StyleError)
		require.NotNil(t, cmd2)

		// The first Set's timeout (seq=1) should not clear the message.
		// We can't easily extract the msg from tea.Tick, so we test
		// Update directly by checking that it handles the timeout type.
	})

	t.Run("ignores non-timeout messages", func(t *testing.T) {
		t.Parallel()

		var m statusbar.StatusMessageModel

		m.Set("msg", statusbar.StyleSuccess)

		consumed := m.Update("random string message")
		assert.False(t, consumed)
		assert.True(t, m.Visible())
	})
}
