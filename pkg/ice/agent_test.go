package ice

import (
	"testing"
)

func TestTimeConsuming(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
}

// func ExampleNew() {
// m := New("a", "a", "b")
// var list []string
// for elem := range m.Iter() {
// 	list = append(list, elem.(string))
// }
// sort.Strings(list)
// fmt.Println(list)
// Output:
// [a a b]
// }
