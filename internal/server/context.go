package server

import "context"

type ctxKey int

const sessionKey ctxKey = 1

func withSession(ctx context.Context, s session) context.Context {
	return context.WithValue(ctx, sessionKey, s)
}

func sessionFrom(r interface{ Context() context.Context }) session {
	v, _ := r.Context().Value(sessionKey).(session)
	return v
}
