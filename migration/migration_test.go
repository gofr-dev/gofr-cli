package migration

import "testing"

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{name: "snake_case", in: "create_employee_table", out: "createEmployeeTable"},
		{name: "kebab_case", in: "create-employee-table", out: "createEmployeeTable"},
		{name: "mixed_caps_snake", in: "Create_Employee_Table", out: "createEmployeeTable"},
		{name: "single_word_lower", in: "employee", out: "employee"},
		{name: "single_word_upper_first", in: "Employee", out: "employee"},
		{name: "no_delimiters_lower", in: "createemployeetable", out: "createemployeetable"},
		{name: "no_delimiters_camel", in: "CreateEmployeeTable", out: "createEmployeeTable"},
		{name: "empty", in: "", out: ""},
		{name: "leading_trailing_delims", in: "_create__employee_table_", out: "createEmployeeTable"},
		{name: "multiple_delims", in: "create---employee___table", out: "createEmployeeTable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toCamelCase(tt.in)
			if got != tt.out {
				t.Fatalf("toCamelCase(%q) = %q; want %q", tt.in, got, tt.out)
			}
		})
	}
}
