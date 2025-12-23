//go:build integration

package infra

import "golang.org/x/crypto/bcrypt"

type bcryptHasher struct{ cost int }

func NewBcryptHasherForIT(cost int) *bcryptHasher {
	if cost <= 0 {
		cost = 12
	}
	return &bcryptHasher{cost: cost}
}

func (b *bcryptHasher) Hash(password string) (string, error) {
	out, err := bcrypt.GenerateFromPassword([]byte(password), b.cost)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (b *bcryptHasher) Compare(hash string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
