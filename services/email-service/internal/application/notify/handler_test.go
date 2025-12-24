package notify

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func testLogger() zerolog.Logger {
	// discard logger output in unit tests
	return zerolog.Nop()
}

func TestService_VerifyEmail_IdempotentSkip_DoesNotSend(t *testing.T) {
	ctx := context.Background()

	sender := &fakeSender{}
	idem := newFakeIdem()

	ttl := 24 * time.Hour
	svc := NewService(sender, idem, ttl, testLogger())

	link := "http://localhost:8090/verify?token=abc"
	key := "email:verify:abc"
	idem.SetSeen(key, true)

	if err := svc.VerifyEmail(ctx, "u1", "a@b.com", link); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if sender.VerifyCalls() != 0 {
		t.Fatalf("expected sender NOT called, got %d", sender.VerifyCalls())
	}
	if idem.SeenCalls() != 1 {
		t.Fatalf("expected Seen called once, got %d", idem.SeenCalls())
	}
	if idem.MarkCalls() != 0 {
		t.Fatalf("expected MarkSent NOT called, got %d", idem.MarkCalls())
	}
}

func TestService_VerifyEmail_SeenError_ReturnsErrorAndDoesNotSend(t *testing.T) {
	ctx := context.Background()

	sender := &fakeSender{}
	idem := newFakeIdem()
	idem.SetSeenErr(errBoom)

	svc := NewService(sender, idem, 24*time.Hour, testLogger())

	link := "http://localhost:8090/verify?token=abc"
	err := svc.VerifyEmail(ctx, "u1", "a@b.com", link)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if sender.VerifyCalls() != 0 {
		t.Fatalf("expected sender NOT called, got %d", sender.VerifyCalls())
	}
	if idem.MarkCalls() != 0 {
		t.Fatalf("expected MarkSent NOT called, got %d", idem.MarkCalls())
	}
}

func TestService_VerifyEmail_SendSuccess_ThenMarkSuccess(t *testing.T) {
	ctx := context.Background()

	sender := &fakeSender{}
	idem := newFakeIdem()

	ttl := 2 * time.Hour
	svc := NewService(sender, idem, ttl, testLogger())

	link := "http://localhost:8090/verify?token=abc"
	key := "email:verify:abc"

	if err := svc.VerifyEmail(ctx, "u1", "a@b.com", link); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if sender.VerifyCalls() != 1 {
		t.Fatalf("expected sender called once, got %d", sender.VerifyCalls())
	}
	if idem.SeenCalls() != 1 {
		t.Fatalf("expected Seen called once, got %d", idem.SeenCalls())
	}
	if idem.MarkCalls() != 1 {
		t.Fatalf("expected MarkSent called once, got %d", idem.MarkCalls())
	}
	// verify key marked
	seen, _ := idem.Seen(ctx, key)
	if !seen {
		t.Fatalf("expected key marked sent")
	}
}

func TestService_VerifyEmail_SendTemporaryError_ReturnsError_NoMark(t *testing.T) {
	ctx := context.Background()

	sender := &fakeSender{}
	sender.SetVerifyErr(TemporaryError{msg: "temp"})
	idem := newFakeIdem()

	svc := NewService(sender, idem, 24*time.Hour, testLogger())

	link := "http://localhost:8090/verify?token=abc"
	err := svc.VerifyEmail(ctx, "u1", "a@b.com", link)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if sender.VerifyCalls() != 1 {
		t.Fatalf("expected sender called once, got %d", sender.VerifyCalls())
	}
	if idem.MarkCalls() != 0 {
		t.Fatalf("expected MarkSent NOT called, got %d", idem.MarkCalls())
	}
}

func TestService_VerifyEmail_SendPermanentError_ReturnsError_NoMark(t *testing.T) {
	ctx := context.Background()

	sender := &fakeSender{}
	sender.SetVerifyErr(PermanentError{msg: "perm"})
	idem := newFakeIdem()

	svc := NewService(sender, idem, 24*time.Hour, testLogger())

	link := "http://localhost:8090/verify?token=abc"
	err := svc.VerifyEmail(ctx, "u1", "a@b.com", link)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if sender.VerifyCalls() != 1 {
		t.Fatalf("expected sender called once, got %d", sender.VerifyCalls())
	}
	if idem.MarkCalls() != 0 {
		t.Fatalf("expected MarkSent NOT called, got %d", idem.MarkCalls())
	}
}

func TestService_VerifyEmail_SendSuccess_MarkFails_ReturnsNil(t *testing.T) {
	ctx := context.Background()

	sender := &fakeSender{}
	idem := newFakeIdem()
	idem.SetMarkErr(errBoom)

	svc := NewService(sender, idem, 24*time.Hour, testLogger())

	link := "http://localhost:8090/verify?token=abc"
	err := svc.VerifyEmail(ctx, "u1", "a@b.com", link)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if sender.VerifyCalls() != 1 {
		t.Fatalf("expected sender called once, got %d", sender.VerifyCalls())
	}
	if idem.MarkCalls() != 1 {
		t.Fatalf("expected MarkSent called once, got %d", idem.MarkCalls())
	}
}

func TestService_PasswordReset_IdempotentSkip_DoesNotSend(t *testing.T) {
	ctx := context.Background()

	sender := &fakeSender{}
	idem := newFakeIdem()
	svc := NewService(sender, idem, 24*time.Hour, testLogger())

	link := "http://localhost:8090/reset?token=xyz"
	key := "email:reset:xyz"
	idem.SetSeen(key, true)

	if err := svc.PasswordReset(ctx, "u1", "a@b.com", link); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if sender.ResetCalls() != 0 {
		t.Fatalf("expected sender NOT called, got %d", sender.ResetCalls())
	}
	if idem.SeenCalls() != 1 {
		t.Fatalf("expected Seen called once, got %d", idem.SeenCalls())
	}
	if idem.MarkCalls() != 0 {
		t.Fatalf("expected MarkSent NOT called, got %d", idem.MarkCalls())
	}
}

func TestService_PasswordReset_SendSuccess_ThenMarkSuccess(t *testing.T) {
	ctx := context.Background()

	sender := &fakeSender{}
	idem := newFakeIdem()
	svc := NewService(sender, idem, 30*time.Minute, testLogger())

	link := "http://localhost:8090/reset?token=xyz"
	key := "email:reset:xyz"

	if err := svc.PasswordReset(ctx, "u1", "a@b.com", link); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if sender.ResetCalls() != 1 {
		t.Fatalf("expected sender called once, got %d", sender.ResetCalls())
	}
	if idem.SeenCalls() != 1 {
		t.Fatalf("expected Seen called once, got %d", idem.SeenCalls())
	}
	if idem.MarkCalls() != 1 {
		t.Fatalf("expected MarkSent called once, got %d", idem.MarkCalls())
	}

	seen, _ := idem.Seen(ctx, key)
	if !seen {
		t.Fatalf("expected key marked sent")
	}
}

func TestService_PasswordReset_SendFails_NoMark(t *testing.T) {
	ctx := context.Background()

	sender := &fakeSender{}
	sender.SetResetErr(TemporaryError{msg: "temp"})
	idem := newFakeIdem()
	svc := NewService(sender, idem, 24*time.Hour, testLogger())

	link := "http://localhost:8090/reset?token=xyz"
	err := svc.PasswordReset(ctx, "u1", "a@b.com", link)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if sender.ResetCalls() != 1 {
		t.Fatalf("expected sender called once, got %d", sender.ResetCalls())
	}
	if idem.MarkCalls() != 0 {
		t.Fatalf("expected MarkSent NOT called, got %d", idem.MarkCalls())
	}
}

func TestService_PasswordReset_SendSuccess_MarkFails_ReturnsNil(t *testing.T) {
	ctx := context.Background()

	sender := &fakeSender{}
	idem := newFakeIdem()
	idem.SetMarkErr(errBoom)

	svc := NewService(sender, idem, 24*time.Hour, testLogger())

	link := "http://localhost:8090/reset?token=xyz"
	err := svc.PasswordReset(ctx, "u1", "a@b.com", link)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if sender.ResetCalls() != 1 {
		t.Fatalf("expected sender called once, got %d", sender.ResetCalls())
	}
	if idem.MarkCalls() != 1 {
		t.Fatalf("expected MarkSent called once, got %d", idem.MarkCalls())
	}
}
