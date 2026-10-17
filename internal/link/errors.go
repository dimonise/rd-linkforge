package link

import "fmt"

type AliasError struct {
    Alias string
    Err   error
}

func (e *AliasError) Error() string {
    return fmt.Sprintf("link: alias %q: %v", e.Alias, e.Err)
}

func (e *AliasError) Unwrap() error {
    return e.Err
}