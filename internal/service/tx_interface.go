package service

import "context"

type txManager interface {
	Run(ctx context.Context, fn func(ctx context.Context) error) error
}
