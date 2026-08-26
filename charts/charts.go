// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0
//

package charts

import (
	"embed"
)

// InternalChart embeds the internal charts in embed.FS
//
//go:embed internal
var InternalChart embed.FS

// InternalChartsPath ist the internal charts path
const InternalChartsPath = "internal"
