//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

package shared

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/atomic"
)

type waiter struct {
	log           logr.Logger
	start         time.Time
	period        time.Duration
	message       atomic.String
	keysAndValues []any
	done          chan struct{}
}

// InformOnWaiting does
func InformOnWaiting(log logr.Logger, period time.Duration, message string, keysAndValues ...any) *waiter {
	w := &waiter{
		log:           log,
		start:         time.Now(),
		period:        period,
		keysAndValues: keysAndValues,
		done:          make(chan struct{}),
	}
	w.message.Store(message)
	go w.run()
	return w
}

func (w *waiter) UpdateMessage(message string) {
	w.message.Store(message)
}

func (w *waiter) run() {
	ticker := time.NewTicker(w.period)
	defer ticker.Stop()
	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			delta := int(time.Since(w.start).Seconds())
			w.log.Info(fmt.Sprintf("%s [%ds]", w.message.Load(), delta), w.keysAndValues...)
		}
	}
}

func (w *waiter) Done(err error) {
	w.done <- struct{}{}
	if err != nil {
		w.log.Info("failed: " + err.Error())
	} else {
		w.log.Info("succeeded")
	}
}