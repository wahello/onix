//   Onix Config DatabaseProvider - Dbman
//   Copyright (c) 2018-2020 by www.gatblau.org
//   Licensed under the Apache License, Version 2.0 at http://www.apache.org/licenses/LICENSE-2.0
//   Contributors to this project, hereby assign copyright in this code to the project,
//   to be licensed under the same terms as the rest of the code.
package main

import (
	. "github.com/gatblau/onix/dbman/plugin"
)

// entry point for the PGSQL plugin
func main() {
	ServeDbPlugin("pgsql", new(PgSQLProvider))
}
