package main

type Gd struct {
	Alpha float64
	Beta  float64
	Gamma float64
}

// sum_{n}{ (X[n] - X_n)^2 }/N
func (r *Gd) errf(da, db, dg float64, throughputByLoad []float64) float64 {
	err := float64(0)
	count := int(0)
	for ni := 1; ni < len(throughputByLoad); ni++ {
		n := int64(ni)
		dist := r.X(da, db, dg, n) - throughputByLoad[n]
		err += dist * dist
		count++
	}
	return err / float64(count)
}

// Point in the direction you must go to IMPROVE
// -grad sum_{n}{ (X[n] - X_n)^2 }/N
// -sum_{n} { grad (X[n] - X_n)^2 }/N
// -sum_{n} { 2 (X[n] - X_n)(grad (X_n - X[n])) }/N
// -sum_{n} { 2 (X[n] - X_n)(-grad X[n]) }/N
// -1/N * sum_{n} { (X[n] - X_n) * (dXa, dXb, dXg) }
func (r *Gd) gradErrf(throughputByLoad []float64) (float64, float64, float64) {
	da := float64(0)
	db := float64(0)
	dg := float64(0)
	count := int(0)
	for ni := 1; ni < len(throughputByLoad); ni++ {
		n := int64(ni)
		dist := r.X(0, 0, 0, n) - throughputByLoad[n]
		da += dist * r.dXa(n)
		db += dist * r.dXb(n)
		dg += dist * r.dXg(n)
		count++
	}
	return -da / float64(count), -db / float64(count), -dg / float64(count)
}

// dXa = -(n g) (n-1) (1 + a (n-1) + b n (n-1))^{-2}
// a >= 0
// a <= 1
// b >= 0
// g >= 0
func (r *Gd) dXa(n int64) float64 {
	a := r.Alpha
	b := r.Beta
	g := float64(r.Gamma)
	denominator := (1 + a*float64(n-1) + b*float64(n*(n-1)))
	return -(float64(n) * g) * float64(n-1) / (denominator * denominator)
}

// dXb = -(n g) n (n-1) (1 + a(n-1) + b n (n-1))^{-2}
// a >= 0
// a <= 1
// b >= 0
// g >= 0
func (r *Gd) dXb(n int64) float64 {
	a := r.Alpha
	b := r.Beta
	g := r.Gamma
	denominator := (1 + a*float64(n-1) + b*float64(n*(n-1)))
	return -(float64(n) * g) * float64(n*(n-1)) / (denominator * denominator)
}

// dXg = n / (1 + a(n-1) + b n (n-1))
// g >= 0
func (r *Gd) dXg(n int64) float64 {
	a := r.Alpha
	b := r.Beta
	denominator := (1 + a*float64(n-1) + b*float64(n*(n-1)))
	return float64(n) / denominator
}

// X = n g / (1 + a(n-1) + b n (n-1))
// a >= 0
// a <= 1
// b >= 0
// g >= 0
func (r *Gd) X(da, db, dg float64, n int64) float64 {
	a := r.Alpha + da
	b := r.Beta + db
	g := r.Gamma + dg
	return (float64(n) * g) / (1 + a*float64(n-1) + b*float64(n*(n-1)))
}
