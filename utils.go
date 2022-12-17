package main

// ListEntry identifies types used by list's generics function
type ListEntry interface {
        int | int64 | float64 | string
}

// InList checks item in a list
func InList[T ListEntry](a T, list []T) bool {
        check := 0
        for _, b := range list {
                if b == a {
                        check += 1
                }
        }
        if check != 0 {
                return true
        }
        return false
}

// Set converts input list into set
func Set[T ListEntry](arr []T) []T {
        var out []T
        for _, v := range arr {
                if !InList(v, out) {
                        out = append(out, v)
                }
        }
        return out
}

