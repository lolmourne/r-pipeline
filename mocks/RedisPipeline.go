// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import context "context"
import mock "github.com/stretchr/testify/mock"
import redispipeline "github.com/lolmourne/r-pipeline"

// RedisPipeline is an autogenerated mock type for the RedisPipeline type
type RedisPipeline struct {
	mock.Mock
}

// NewSession provides a mock function with given fields: ctx
func (_m *RedisPipeline) NewSession(ctx context.Context) redispipeline.RedisPipelineSession {
	ret := _m.Called(ctx)

	var r0 redispipeline.RedisPipelineSession
	if rf, ok := ret.Get(0).(func(context.Context) redispipeline.RedisPipelineSession); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(redispipeline.RedisPipelineSession)
		}
	}

	return r0
}