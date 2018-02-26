package models

type Balance struct {
	Available float64
	OnOrders  float64
}

func NewBalance(available float64, onOrders float64) *Balance {
	return &Balance{
		Available: available,
		OnOrders:  onOrders,
	}
}
