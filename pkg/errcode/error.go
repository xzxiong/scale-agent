// Copyright 2024 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errcode

import (
	"errors"
)

var (
	ErrNoResourceRange = errors.New("NoResourceRange")
	ErrNoPodName       = errors.New("NoPodName")
	ErrNoNamespace     = errors.New("NoNamespace")
	ErrNotSupported    = errors.New("NotSupported")
	ErrNotFound        = errors.New("NotFound")
	ErrUnknownEvent    = errors.New("UnknownEvent")
	ErrNeedAlert       = errors.New("NeedAlert")
)

func NoErrOrDie(err error) {
	if err != nil {
		panic(err)
	}
}
