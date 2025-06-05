package types

// ListWithItems combines a List with its items
type ListWithItems struct {
	List  List
	Items []*ListItem
}