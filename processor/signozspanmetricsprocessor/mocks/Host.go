// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	component "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

// Host is an autogenerated mock type for the Host type
type Host struct {
	mock.Mock
}

// GetExporters provides a mock function with given fields:
func (_m *Host) GetExporters() map[pipeline.Signal]map[component.ID]component.Component {
	ret := _m.Called()

	var r0 map[pipeline.Signal]map[component.ID]component.Component
	if rf, ok := ret.Get(0).(func() map[pipeline.Signal]map[component.ID]component.Component); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[pipeline.Signal]map[component.ID]component.Component)
		}
	}

	return r0
}

// GetExtensions provides a mock function with given fields:
func (_m *Host) GetExtensions() map[component.ID]component.Component {
	ret := _m.Called()

	var r0 map[component.ID]component.Component
	if rf, ok := ret.Get(0).(func() map[component.ID]component.Component); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[component.ID]component.Component)
		}
	}

	return r0
}

// GetFactory provides a mock function with given fields: kind, componentType
func (_m *Host) GetFactory(kind component.Kind, componentType component.Type) component.Factory {
	ret := _m.Called(kind, componentType)

	var r0 component.Factory
	if rf, ok := ret.Get(0).(func(component.Kind, component.Type) component.Factory); ok {
		r0 = rf(kind, componentType)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(component.Factory)
		}
	}

	return r0
}

// ReportFatalError provides a mock function with given fields: err
func (_m *Host) ReportFatalError(err error) {
	_m.Called(err)
}
