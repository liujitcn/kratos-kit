package transport

import "github.com/liujitcn/kratos-kit/broker"

type SubscribeOption struct {
	Handler          broker.Handler
	Binder           broker.Binder
	SubscribeOptions []broker.SubscribeOption
}
type SubscribeOptionMap map[string]*SubscribeOption
