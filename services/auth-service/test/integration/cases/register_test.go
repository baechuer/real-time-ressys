//go:build integration

package cases

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	itinfra "github.com/baechuer/real-time-ressys/services/auth-service/test/integration/infra"
)

func Test_Register_CreatesUser(t *testing.T) {
	env, err := itinfra.LoadEnv()
	require.NoError(t, err)

	d := MustNewDeps(t, env)
	defer d.Close(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	require.NoError(t, itinfra.ResetAll(ctx, d.DB, d.RDB))

	reg, err := d.Svc.Register(ctx, "it_reg@example.com", "StrongPassw0rd!!")
	require.NoError(t, err)
	require.NotEmpty(t, reg.User.ID)
	require.Equal(t, "it_reg@example.com", reg.User.Email)
}
