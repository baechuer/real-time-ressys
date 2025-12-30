package notify

import "context"

type FakeUserResolver struct {
	Email string
	Err   error
}

func (f *FakeUserResolver) GetEmail(ctx context.Context, userID string) (string, error) {
	return f.Email, f.Err
}
