// Copyright ©2011-2012 The bíogo.matrix Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package matrix

import (
	"code.google.com/p/biogo.blas"
	"fmt"
	"math"
	"math/rand"
)

// additions

import (
    "encoding/binary"
    xxh "bitbucket.org/StephaneBunel/xxhash-go"
    )

type VectorFilterFunc func([]float64) bool
type VectorApplyFunc func([]float64) []float64

func (d *Dense) AppendDense(b, c *Dense) *Dense {
    if d.cols != b.cols {
        panic(ErrColLength)
    }
    c = c.reallocate(d.rows+b.rows, d.cols)
    copy(c.matrix[0:d.cols*d.rows], d.matrix)
    copy(c.matrix[d.cols*d.rows:d.cols*d.rows+b.cols*b.rows], b.matrix)

    return c
}

func AppendDense(d ...*Dense) *Dense {
    var r int
    c := d[0].cols
    for _, dens := range d {
        if dens.cols != c {
            panic(ErrColLength)
        }
        r += dens.rows
    }
    newDens := &Dense{
        rows: r,
        cols: c,
        matrix: make(denseRow,r*c),
    }
    r = 0
    for _, dens := range d {
        copy(newDens.matrix[r*newDens.cols:r*newDens.cols+dens.rows*dens.cols],dens.matrix)
        r += dens.rows
    }
    return newDens
}

func (d *Dense) VectorFilterDense(f VectorFilterFunc) *Dense {
    c := &Dense{
        rows:d.rows,
        cols:d.cols,
        matrix: make(denseRow,d.rows*d.cols),
    }
    r := 0
    for i:=0; i < d.rows; i++ {
        if f(d.matrix[i*d.cols:i*d.cols+d.rows]) {
            r++
            copy(c.matrix[i*d.cols:(i+1)*d.cols],d.matrix[i*d.cols:(i+1)*d.cols])
        }
    }
    c = c.reallocate(r, c.cols)
    return c
}

//given column #s, create new Denses based on unique combinations
func (d *Dense) SplitDense(cols []int) map[uint32]*Dense {
    combos := make(map[uint32]*Dense,0)

    for i:=0; i < d.rows; i++ {
        g := make([]byte,len(cols)*8)
        for c_index, c := range cols {
            binary.BigEndian.PutUint64(g[c_index*8:(c_index+1)*8],math.Float64bits(d.matrix[i*d.cols+c]))
        }
        group := xxh.Checksum32(g)
        if _, ok := combos[group]; ok {
            n := &Dense{
                rows: combos[group].rows + 1,
                cols: combos[group].cols,
                matrix: make(denseRow, (combos[group].rows + 1)*combos[group].cols),
            }
            combos[group] = combos[group].FillDense(n)
            copy(combos[group].matrix[(combos[group].rows-1)*combos[group].cols:combos[group].rows*combos[group].cols],d.matrix[i*d.cols:(i+1)*d.cols])
        } else {
            combos[group] = &Dense{
                rows: 1,
                cols: d.cols,
                matrix: make(denseRow, d.cols),
            }
            copy(combos[group].matrix[0:d.cols],d.matrix[i*d.cols:(i+1)*d.cols])
        }
    }

    return combos
}

func (d *Dense) VectorApplyDense(f VectorApplyFunc) *Dense {
    var r int
    newRows := make([]float64,0)
	for i:=0; i < d.rows; i++ {
	    applied := f(d.matrix[i*d.cols:(i+1)*d.cols])
	    if applied != nil {
	        r++
            newRows = append(newRows,applied...)
        }
	}
    den := &Dense{
        rows:r,
        cols:len(newRows)/r,
        matrix: denseRow(newRows),
    }
	return den
}

func (d *Dense) MaxFrom(index int, col bool) (float64,int) {
    var n float64
    var oindex int
    if !col {
        if index < 0 || index >= d.rows {
            panic(ErrColLength)
        }
        for i:=0; i < d.cols; i++ {
            tempn := n
            n = math.Max(d.at(index,i),n)
            if tempn != n {
                oindex = i
            }
        }
    } else {
        if index < 0 || index >= d.cols {
            panic(ErrRowLength)
        }
        for i:=0; i < d.rows; i++ {
            tempn := n
            n = math.Max(d.at(i,index),n)
            if tempn != n {
                oindex = i
            }
        }
    }
    return n, oindex
}

func (d *Dense) FillDense(c *Dense) *Dense {
    if d.rows*d.cols > c.rows*c.cols {
        panic(ErrRowLength)
    }
    copy(c.matrix[:d.rows*d.cols],d.matrix[:d.rows*d.cols])
    return c
}

func (d *Dense) ToFloatSlice() [][]float64 {
    f := make([][]float64,d.rows)
    for i:=0; i<d.rows; i++ {
        r := make([]float64,d.cols)
        copy(r,d.matrix[i*d.cols:(i+1)*d.cols])
        f[i] = r
    }
    return f
}

// end additions

var blasEngine blas.Blas

// Type Dense represents a dense matrix.
type Dense struct {
	Margin     int // The number of cells in from the edge of the matrix to format.
	rows, cols int
	matrix     denseRow
}

// Type UnsafeDense represents the matrix data stored in a Dense with no arithmetic methods associated,
// but with the structure exposed. This type allows more low level operations to be constructed and
// interconversion with other matrix formats.
type UnsafeDense struct {
	Rows, Cols int
	Data       []float64
	Stride     int
}

// Dense returns a dense matrix, checking that dimensions are valid.
func (u UnsafeDense) Dense() (*Dense, error) {
	if u.Rows*u.Cols != len(u.Data) {
		return nil, ErrShape
	}
	if u.Stride != u.Cols { // While submatrices do not yet exist.
		return nil, ErrIllegalStride
	}
	return &Dense{
		rows:   u.Rows,
		cols:   u.Cols,
		matrix: u.Data,
	}, nil
}

// Unsafe returns a shallow copy the dense matrix as an UnsafeDense. Changes to the stride and
// matrix dimensions are not reflected in the original matrix, however changes in the Data slice
// are reflected in the original matrix.
func (d *Dense) Unsafe() UnsafeDense {
	return UnsafeDense{
		Rows:   d.rows,
		Cols:   d.cols,
		Stride: d.cols,
		Data:   d.matrix,
	}
}

// A DensePanicker is a function that returns a dense matrix and may panic.
type DensePanicker func() *Dense

// MaybeDense will recover a panic with a type matrix.Error from fn, and return this error.
// Any other error is re-panicked.
func MaybeDense(fn DensePanicker) (d *Dense, err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			if err, ok = r.(Error); ok {
				return
			}
			panic(r)
		}
	}()
	return fn(), nil
}

// MustDense can be used to wrap a function returning a dense matrix and an error.
// If the returned error is not nil, MustDense will panic.
func MustDense(d *Dense, err error) *Dense {
	if err != nil {
		panic(err)
	}
	return d
}

func (d *Dense) reallocate(r, c int) *Dense {
	if d == nil {
		d = &Dense{
			rows:   r,
			cols:   c,
			matrix: make(denseRow, r*c),
		}
	} else {
		l := r * c
		if cap(d.matrix) < l {
			d.matrix = make(denseRow, l)
		} else {
			d.matrix = d.matrix[:l]
		}
		d.rows = r
		d.cols = c
	}
	return d
}

// NewDense returns a dense matrix based on a slice of float64 slices. An error is returned
// if either dimension is zero or rows are not of equal length.
func NewDense(a [][]float64) (*Dense, error) {
	if len(a) == 0 || len(a[0]) == 0 {
		return nil, ErrZeroLength
	}

	m := Dense{
		rows: len(a),
		cols: len(a[0]),
	}
	for _, row := range a {
		if len(row) != m.cols {
			return nil, ErrRowLength
		}
	}
	m.matrix = make(denseRow, len(a)*len(a[0]))

	for i, row := range a {
		copy(m.matrix[i*m.cols:(i+1)*m.cols], row)
	}

	return &m, nil
}

// New returns a new dense r by c matrix.
func (d *Dense) New(r, c int) (Matrix, error) {
	return ZeroDense(r, c)
}

// ZeroDense returns an r row by c column O matrix. An error is returned if either dimension
// is zero.
func ZeroDense(r, c int) (*Dense, error) {
	if r < 1 || c < 1 {
		return nil, ErrZeroLength
	}

	return &Dense{
		rows:   r,
		cols:   c,
		matrix: make(denseRow, r*c),
	}, nil
}

// IdentityDense returns the a size by size I matrix. An error is returned if size is zero.
func IdentityDense(size int) (*Dense, error) {
	if size < 1 {
		return nil, ErrZeroLength
	}

	m := &Dense{
		rows:   size,
		cols:   size,
		matrix: make(denseRow, size*size),
	}

	for i := 0; i < size; i++ {
		m.matrix[i*size+i] = 1
	}

	return m, nil
}

// FuncDense returns a dense matrix filled with the returned values of fn with a matrix density of rho.
// An error is returned if either dimension is zero.
func FuncDense(r, c int, rho float64, fn FloatFunc) (*Dense, error) {
	if r < 1 || c < 1 {
		return nil, ErrZeroLength
	}

	m := &Dense{
		rows:   r,
		cols:   c,
		matrix: make(denseRow, r*c),
	}

	for i := range m.matrix {
		if rand.Float64() < rho {
			m.matrix[i] = fn()
		}
	}

	return m, nil
}

// ElementsDense returns the elements of mats concatenated, row-wise, into a row vector.
func ElementsDense(mats ...Matrix) *Dense {
	var length int
	for _, m := range mats {
		switch m := m.(type) {
		case *Dense:
			length += len(m.matrix)
		}
	}

	t := make(denseRow, 0, length)
	for _, m := range mats {
		switch m := m.(type) {
		case *Dense:
			t = append(t, m.matrix...)
		case Matrix:
			rows, cols := m.Dims()
			for r := 0; r < rows; r++ {
				for c := 0; c < cols; c++ {
					t = append(t, m.At(r, c))
				}
			}
		}
	}

	e := &Dense{
		rows:   1,
		cols:   length,
		matrix: t,
	}

	return e
}

// ElementsVector returns the matrix's elements concatenated, row-wise, into a float slice.
func (d *Dense) ElementsVector() []float64 {
	return append([]float64(nil), d.matrix...)
}

// Clone returns a copy of the matrix.
func (d *Dense) Clone(c Matrix) Matrix {
	cc, _ := c.(*Dense)
	return d.CloneDense(cc)
}

// Clone returns a copy of the matrix, retaining its concrete type.
func (d *Dense) CloneDense(c *Dense) *Dense {
	c = c.reallocate(d.Dims())
	copy(c.matrix, d.matrix)
	return c
}

// Dense returns the matrix as a Dense. The returned matrix is not a copy.
func (d *Dense) Dense(_ *Dense) *Dense { return d }

// Sparse returns a copy of the matrix represented as a Sparse.
func (d *Dense) Sparse(s *Sparse) *Sparse {
	s = s.reallocate(d.Dims())

	for r := 0; r < d.rows; r++ {
		s.matrix[r] = s.matrix[r][:0]
		for c := 0; c < d.cols; c++ {
			if v := d.at(r, c); v != 0 {
				s.matrix[r] = append(s.matrix[r], sparseElem{index: c, value: v})
			}
		}
	}

	return s
}

// Dims return the dimensions of the matrix.
func (d *Dense) Dims() (r, c int) {
	return d.rows, d.cols
}

// Reshape, returns a shallow copy of with the dimensions set to r and c. Reshape will
// panic with ErrShape if r x c does not equal the number of elements in the matrix.
func (d *Dense) Reshape(r, c int) Matrix { return d.ReshapeDense(r, c) }

// ReshapeDense, returns a shallow copy of with the dimensions set to r and c, retaining the concrete
// type of the matrix. ReshapeDense will panic with ErrShape if r x c does not equal the number of
// elements in the matrix.
func (d *Dense) ReshapeDense(r, c int) *Dense {
	if r*c != d.rows*d.cols {
		panic(ErrShape)
	}
	return &Dense{
		rows:   r,
		cols:   c,
		matrix: d.matrix,
	}
}

// Det returns the determinant of the matrix.
// TODO: implement
func (d *Dense) Det() float64 {
	panic("not implemented")
}

// Min returns the value of the minimum element value of the matrix.
func (d *Dense) Min() float64 {
	return d.matrix.min()
}

// Max returns the value of the maximum element value of the matrix.
func (d *Dense) Max() float64 {
	return d.matrix.max()
}

// Set sets the value of the element at (r, c) to v. Set will panic with ErrIndexOutOfRange
// if r or c are not legal indices.
func (d *Dense) Set(r, c int, v float64) {
	if r >= d.rows || c >= d.cols || r < 0 || c < 0 {
		panic(ErrIndexOutOfRange)
	}

	d.set(r, c, v)
}

func (d *Dense) set(r, c int, v float64) {
	d.matrix[r*d.cols+c] = v
}

// At return the value of the element at (r, c). At will panic with ErrIndexOutOfRange if
// r or c are not legal indices.
func (d *Dense) At(r, c int) (v float64) {
	if r >= d.rows || c >= d.cols || c < 0 || r < 0 {
		panic(ErrIndexOutOfRange)
	}
	return d.at(r, c)
}

func (d *Dense) at(r, c int) float64 {
	return d.matrix[r*d.cols+c]
}

// Column returns a slice of float64 that is a copy of the values at column c of the matrix.
// Column will panic with ErrIndexOutOfRange is c is not a valid column index.
func (d *Dense) Column(c int) []float64 {
	if c >= d.cols || c < 0 {
		panic(ErrIndexOutOfRange)
	}
	col := make([]float64, d.rows)
	blasEngine.Dcopy(d.rows, d.matrix[c:], d.cols, col, 1)
	return col
}

// SetColumn sets the values at column c of the matrix to the values of the slice v. SetColumn
// will panic with ErrIndexOutOfRange is c is not a valid column index and ErrColLength if the
// length of v does not match the matrix column length.
func (d *Dense) SetColumn(c int, v []float64) {
	if c >= d.cols || c < 0 {
		panic(ErrIndexOutOfRange)
	}
	if len(v) != d.rows {
		panic(ErrColLength)
	}
	blasEngine.Dcopy(d.rows, v, 1, d.matrix[c:], d.cols)
}

// Row returns a slice of float64 that is a copy of the values at row r of the matrix.
// Row will panic with ErrIndexOutOfRange is r is not a valid row index.
func (d *Dense) Row(r int) []float64 {
	if r >= d.rows || r < 0 {
		panic(ErrIndexOutOfRange)
	}
	row := make([]float64, d.cols)
	blasEngine.Dcopy(d.cols, d.matrix[r*d.cols:], 1, row, 1)
	return row
}

// SetRow sets the values at row r of the matrix to the values of the slice v. SetRow will panic
// with ErrIndexOutOfRange is r is not a valid row index and ErrRowLength if the length of v does
// not match the matrix row length.
func (d *Dense) SetRow(r int, v []float64) {
	if r >= d.rows || r < 0 {
		panic(ErrIndexOutOfRange)
	}
	if len(v) != d.cols {
		panic(ErrRowLength)
	}
	blasEngine.Dcopy(d.cols, v, 1, d.matrix[r*d.cols:], 1)
}

// Trace returns the trace of a square matrix. Trace will panic with ErrSquare if the matrix
// is not square.
func (d *Dense) Trace() float64 {
	if d.rows != d.cols {
		panic(ErrSquare)
	}
	var t float64
	for i := 0; i < len(d.matrix); i += d.cols + 1 {
		t += d.matrix[i]
	}
	return t
}

// Norm returns a variety of norms for the matrix.
//
// Valid ord values are:
//
// 	          1 - max of the sum of the absolute values of columns
// 	         -1 - min of the sum of the absolute values of columns
// 	 matrix.Inf - max of the sum of the absolute values of rows
// 	-matrix.Inf - min of the sum of the absolute values of rows
// 	 matrix.Fro - Frobenius norm (0 is an alias to this)
//
// Norm will panic with ErrNormOrder if an illegal norm order is specified.
func (d *Dense) Norm(ord int) float64 {
	var n float64
	if ord == 0 {
		ord = Fro
	}
	switch ord {
	case 2, -2:
		panic("not implemented - feel free to port an svd function to matrix")
	case 1:
		sum := d.SumAxis(Cols)
		for _, e := range sum.matrix {
			n = math.Max(math.Abs(e), n)
		}
	case Inf:
		sum := d.SumAxis(Rows)
		for _, e := range sum.matrix {
			n = math.Max(math.Abs(e), n)
		}
	case -1:
		n = math.MaxFloat64
		sum := d.SumAxis(Cols)
		for _, e := range sum.matrix {
			n = math.Min(math.Abs(e), n)
		}
	case -Inf:
		n = math.MaxFloat64
		sum := d.SumAxis(Rows)
		for _, e := range sum.matrix {
			n = math.Min(math.Abs(e), n)
		}
	case Fro:
		for _, e := range d.matrix {
			n += e * e
		}
		return math.Sqrt(n)
	default:
		panic(ErrNormOrder)
	}

	return n
}

// SumAxis return a column or row vector holding the sums of rows or columns.
func (d *Dense) SumAxis(cols bool) *Dense {
	m := &Dense{}
	if !cols {
		m.rows, m.cols, m.matrix = d.rows, 1, make(denseRow, d.rows)
		for i := 0; i < d.rows; i++ {
			row := d.matrix[i*d.cols : (i+1)*d.cols]
			m.matrix[i] = row.sum()
		}
	} else {
		m.rows, m.cols, m.matrix = 1, d.cols, make(denseRow, d.cols)
		for i := 0; i < d.cols; i++ {
			var n float64
			for j := 0; j < d.rows; j++ {
				n += d.at(j, i)
			}
			m.matrix[i] = n
		}
	}

	return m
}

// MaxAxis return a column or row vector holding the maximum of rows or columns.
func (d *Dense) MaxAxis(cols bool) *Dense {
	m := &Dense{}
	if !cols {
		m.rows, m.cols, m.matrix = d.rows, 1, make(denseRow, d.rows)
		for i := 0; i < d.rows; i++ {
			row := d.matrix[i*d.cols : (i+1)*d.cols]
			m.matrix[i] = row.max()
		}
	} else {
		m.rows, m.cols, m.matrix = 1, d.cols, make(denseRow, d.cols)
		for i := 0; i < d.cols; i++ {
			var n float64
			for j := 0; j < d.rows; j++ {
				n = math.Max(d.at(j, i), n)
			}
			m.matrix[i] = n
		}
	}

	return m
}

// MinAxis return a column or row vector holding the minimum of rows or columns.
func (d *Dense) MinAxis(cols bool) *Dense {
	m := &Dense{}
	if !cols {
		m.rows, m.cols, m.matrix = d.rows, 1, make(denseRow, d.rows)
		for i := 0; i < d.rows; i++ {
			row := d.matrix[i*d.cols : (i+1)*d.cols]
			m.matrix[i] = row.min()
		}
	} else {
		m.rows, m.cols, m.matrix = 1, d.cols, make(denseRow, d.cols)
		for i := 0; i < d.cols; i++ {
			var n = math.MaxFloat64
			for j := 0; j < d.rows; j++ {
				n = math.Min(d.at(j, i), n)
			}
			m.matrix[i] = n
		}
	}

	return m
}

// U returns the upper triangular matrix of the matrix. U will panic with ErrSquare if the matrix is not
// square.
func (d *Dense) U(c Matrix) Matrix {
	cc, _ := c.(*Dense)
	return d.UDense(cc)
}

// UDense returns the upper triangular matrix of the matrix retaining the concrete type of the matrix.
// UDense will panic with ErrSquare if the matrix is not square.
func (d *Dense) UDense(c *Dense) *Dense {
	if d.rows != d.cols {
		panic(ErrSquare)
	}
	if c == d {
		for i := 1; i < d.rows; i++ {
			d.matrix[i*d.cols : i*d.cols+i].zero()
		}
		return d
	}
	c = c.reallocate(d.Dims())
	for i := 0; i < d.rows; i++ {
		copy(c.matrix[i*d.cols+i:(i+1)*d.cols], d.matrix[i*d.cols+i:(i+1)*d.cols])
	}
	return c
}

// L returns the lower triangular matrix of the matrix. L will panic with ErrSquare if the matrix is not
// square.
func (d *Dense) L(c Matrix) Matrix {
	cc, _ := c.(*Dense)
	return d.LDense(cc)
}

// LDense returns the lower triangular matrix of the matrix retaining the concrete type of the matrix.
// LDense will panic with ErrSquare if the matrix is not square.
func (d *Dense) LDense(c *Dense) *Dense {
	if d.rows != d.cols {
		panic(ErrSquare)
	}
	if c == d {
		for i := 0; i < d.rows-1; i++ {
			d.matrix[i*d.cols+i+1 : (i+1)*d.cols].zero()
		}
		return d
	}
	c = c.reallocate(d.Dims())
	for i := 0; i < d.rows; i++ {
		copy(c.matrix[i*d.cols:i*d.cols+i+1], d.matrix[i*d.cols:i*d.cols+i+1])
	}
	return c
}

// T returns the transpose of the matrix.
func (d *Dense) T(c Matrix) Matrix {
	cc, _ := c.(*Dense)
	return d.TDense(cc)
}

// TDense returns the transpose of the matrix retaining the concrete type of the matrix.
func (d *Dense) TDense(c *Dense) *Dense {
	if d.rows == 1 || d.cols == 1 {
		c = d.CloneDense(c)
		c.rows, c.cols = c.cols, c.rows
		return c
	}

	cols, rows := d.Dims()
	if c == d {
		c = nil
	}
	c = c.reallocate(rows, cols)
	for i := 0; i < d.cols; i++ {
		for j := 0; j < d.rows; j++ {
			c.set(i, j, d.at(j, i))
		}
	}

	return c
}

// Add returns the sum of the matrix and the parameter. Add will panic with ErrShape if the
// two matrices do not have the same dimensions.
func (d *Dense) Add(b, c Matrix) Matrix {
	cc, _ := c.(*Dense)
	switch b := b.(type) {
	case *Dense:
		return d.AddDense(b, cc)
	case *Sparse:
		return d.addSparse(b, cc)
	case *Pivot:
		return d.addPivot(b, cc)
	default:
		panic("not implemented")
	}

	panic("cannot reach")
}

// AddDense returns a dense matrix which is the sum of the matrix and the parameter. AddDense will
// panic with ErrShape if the two matrices do not have the same dimensions.
func (d *Dense) AddDense(b, c *Dense) *Dense {
	if d.rows != b.rows || d.cols != b.cols {
		panic(ErrShape)
	}

	if c != d && c != b {
		c = c.reallocate(d.Dims())
	}
	c.matrix = d.matrix.foldAdd(b.matrix, c.matrix)

	return c
}

func (d *Dense) addSparse(b *Sparse, c *Dense) *Dense {
	if d.rows != b.rows || d.cols != b.cols {
		panic(ErrShape)
	}

	if c != d {
		c = c.reallocate(d.Dims())
		copy(c.matrix, d.matrix)
	}
	for r, row := range b.matrix {
		for _, e := range row {
			c.matrix[r*c.cols+e.index] += e.value
		}
	}

	return c
}

func (d *Dense) addPivot(b *Pivot, c *Dense) *Dense {
	if d.rows != len(b.matrix) || d.cols != len(b.matrix) {
		panic(ErrShape)
	}

	if c != d {
		c = c.reallocate(d.Dims())
		copy(c.matrix, d.matrix)
	}

	for row, col := range b.xirtam {
		c.matrix[row*c.cols+col]++
	}

	return c
}

// Sub returns the result of subtraction of the parameter from the matrix. Sub will panic with ErrShape
// if the two matrices do not have the same dimensions.
func (d *Dense) Sub(b, c Matrix) Matrix {
	cc, _ := c.(*Dense)
	switch b := b.(type) {
	case *Dense:
		return d.SubDense(b, cc)
	case *Sparse:
		return d.subSparse(b, cc)
	case *Pivot:
		return d.subPivot(b, cc)
	default:
		panic("not implemented")
	}

	panic("cannot reach")
}

// SubDense returns the result a dense matrix which is the result of subtraction of the parameter from the matrix.
// SubDense will panic with ErrShape if the two matrices do not have the same dimensions.
func (d *Dense) SubDense(b, c *Dense) *Dense {
	if d.rows != b.rows || d.cols != b.cols {
		panic(ErrShape)
	}
	if c != d && c != b {
		c = c.reallocate(d.Dims())
	}
	c.matrix = d.matrix.foldSub(b.matrix, c.matrix)

	return c
}

func (d *Dense) subSparse(b *Sparse, c *Dense) *Dense {
	if d.rows != b.rows || d.cols != b.cols {
		panic(ErrShape)
	}

	if c != d {
		c = c.reallocate(d.Dims())
		copy(c.matrix, d.matrix)
	}
	for r, row := range b.matrix {
		for _, e := range row {
			c.matrix[r*c.cols+e.index] -= e.value
		}
	}

	return c
}

func (d *Dense) subPivot(b *Pivot, c *Dense) *Dense {
	if d.rows != len(b.matrix) || d.cols != len(b.matrix) {
		panic(ErrShape)
	}

	if c != d {
		c = c.reallocate(d.Dims())
		copy(c.matrix, d.matrix)
	}

	for row, col := range b.xirtam {
		c.matrix[row*c.cols+col]--
	}

	return c
}

// MulElem returns the element-wise multiplication of the matrix and the parameter. MulElem will panic with ErrShape
// if the two matrices do not have the same dimensions.
func (d *Dense) MulElem(b, c Matrix) Matrix {
	cc, _ := c.(*Dense)
	switch b := b.(type) {
	case *Dense:
		return d.MulElemDense(b, cc)
	case *Sparse:
		return d.mulElemSparse(b, cc)
	case *Pivot:
		return d.mulElemPivot(b, cc)
	default:
		panic("not implemented")
	}

	panic("cannot reach")
}

// MulElemDense returns a dense matrix which is the result of element-wise multiplication of the matrix and the parameter.
// MulElemDense will panic with ErrShape if the two matrices do not have the same dimensions.
func (d *Dense) MulElemDense(b, c *Dense) *Dense {
	if d.rows != b.rows || d.cols != b.cols {
		panic(ErrShape)
	}

	if c != d && c != b {
		c = c.reallocate(d.Dims())
	}
	c.matrix = d.matrix.foldMul(b.matrix, c.matrix)

	return c
}

func (d *Dense) mulElemSparse(b *Sparse, c *Dense) *Dense {
	if d.rows != b.rows || d.cols != b.cols {
		panic(ErrShape)
	}

	if c != d {
		c = c.reallocate(d.Dims())
		copy(c.matrix, d.matrix)
	}
	var curr, last int
	for r, row := range b.matrix {
		for _, e := range row {
			curr = r*c.cols + e.index
			c.matrix[last:curr].zero()
			c.matrix[curr] *= e.value
			last = curr + 1
		}
	}

	return c
}

func (d *Dense) mulElemPivot(b *Pivot, c *Dense) *Dense {
	if d.rows != len(b.matrix) || d.cols != len(b.matrix) {
		panic(ErrShape)
	}

	vals := make([]float64, len(b.matrix))
	if c != d {
		c = c.reallocate(d.Dims())
	}

	for row, col := range b.xirtam {
		vals[row] = d.matrix[row*d.cols+col]
	}
	c.matrix.zero()
	for row, col := range b.xirtam {
		c.matrix[row*c.cols+col] = vals[row]
	}

	return c
}

// Equals returns the equality of two matrices.
func (d *Dense) Equals(b Matrix) bool {
	switch b := b.(type) {
	case *Dense:
		return d.EqualsDense(b)
	case *Sparse:
		return d.equalsSparse(b)
	case *Pivot:
		return d.equalsPivot(b)
	default:
		panic("not implemented")
	}

	panic("cannot reach")
}

// EqualsDense returns the equality of two dense matrices.
func (d *Dense) EqualsDense(b *Dense) bool {
	if d.rows != b.rows || d.cols != b.cols {
		return false
	}
	return d.matrix.foldEqual(b.matrix)
}

func (d *Dense) equalsSparse(b *Sparse) bool {
	if d.rows != b.rows || d.cols != b.cols {
		return false
	}
	var curr, last int
	for r, row := range b.matrix {
		for _, e := range row {
			curr = r*d.cols + e.index
			for _, v := range d.matrix[last:curr] {
				if v != 0 {
					return false
				}
			}
			if d.matrix[curr] != e.value {
				return false
			}
			last = curr + 1
		}
	}
	return true
}

func (d *Dense) equalsPivot(b *Pivot) bool {
	if d.rows != len(b.matrix) || d.cols != len(b.matrix) {
		return false
	}
	for i, v := range d.matrix {
		if v != 0 && (v != 1 || b.matrix[i%d.cols] != i/d.cols) {
			return false

		}
	}

	return true
}

// EqualsApprox returns the approximate equality of two matrices, tolerance for element-wise equality is
// given by epsilon.
func (d *Dense) EqualsApprox(b Matrix, epsilon float64) bool {
	switch b := b.(type) {
	case *Dense:
		return d.EqualsApproxDense(b, epsilon)
	case *Sparse:
		return d.equalsApproxSparse(b, epsilon)
	case *Pivot:
		return d.equalsApproxPivot(b, epsilon)
	default:
		panic("not implemented")
	}

	panic("cannot reach")
}

// EqualsApproxDense returns the approximate equality of two dense matrices, tolerance for element-wise
// equality is given by epsilon.
func (d *Dense) EqualsApproxDense(b *Dense, epsilon float64) bool {
	if d.rows != b.rows || d.cols != b.cols {
		return false
	}
	return d.matrix.foldApprox(b.matrix, epsilon)
}

func (d *Dense) equalsApproxSparse(b *Sparse, epsilon float64) bool {
	if d.rows != b.rows || d.cols != b.cols {
		return false
	}
	var curr, last int
	for r, row := range b.matrix {
		for _, e := range row {
			curr = r*d.cols + e.index
			for _, v := range d.matrix[last:curr] {
				if math.Abs(v) > epsilon {
					return false
				}
			}
			if math.Abs(d.matrix[curr]-e.value) > epsilon {
				return false
			}
			last = curr + 1
		}
	}
	return true
}

func (d *Dense) equalsApproxPivot(b *Pivot, epsilon float64) bool {
	if d.rows != len(b.matrix) || d.cols != len(b.matrix) {
		return false
	}
	for i, v := range d.matrix {
		if math.Abs(v) > epsilon && (math.Abs(v-1) > epsilon || b.matrix[i%d.cols] != i/d.cols) {
			return false
		}
	}

	return true
}

// Scalar returns the scalar product of the matrix and f.
func (d *Dense) Scalar(f float64, c Matrix) Matrix {
	cc, _ := c.(*Dense)
	return d.ScalarDense(f, cc)
}

// ScalarDense returns the scalar product of the matrix and f as a Dense.
func (d *Dense) ScalarDense(f float64, c *Dense) *Dense {
	if c != d {
		c = c.reallocate(d.Dims())
	}
	c.matrix = d.matrix.scale(f, c.matrix)
	return c
}

// Sum returns the sum of elements in the matrix.
func (d *Dense) Sum() float64 {
	return d.matrix.sum()
}

// Inner returns the sum of element-wise multiplication of the matrix and the parameter. Inner will
// panic with ErrShape if the two matrices do not have the same dimensions.
func (d *Dense) Inner(b Matrix) float64 {
	switch b := b.(type) {
	case *Dense:
		return d.InnerDense(b)
	case *Sparse:
		return d.innerSparse(b)
	case *Pivot:
		return d.innerPivot(b)
	default:
		panic("not implemented")
	}

	panic("cannot reach")
}

// InnerDense returns the sum of element-wise multiplication of the matrix and the parameter.
// InnerDense will panic with ErrShape if the two matrices do not have the same dimensions.
func (d *Dense) InnerDense(b *Dense) float64 {
	if d.rows != b.rows || d.cols != b.cols {
		panic(ErrShape)
	}
	return d.matrix.foldMulSum(b.matrix)
}

func (d *Dense) innerSparse(b *Sparse) float64 {
	if d.rows != b.rows || d.cols != b.cols {
		panic(ErrShape)
	}

	var sum float64
	for r, row := range b.matrix {
		for _, e := range row {
			sum += d.matrix[r*d.cols+e.index] * e.value
		}
	}

	return sum
}

func (d *Dense) innerPivot(b *Pivot) float64 {
	if d.rows != len(b.matrix) || d.cols != len(b.matrix) {
		panic(ErrShape)
	}

	var sum float64
	for row, col := range b.xirtam {
		sum += d.matrix[row*d.cols+col]
	}

	return sum
}

// Dot returns the matrix product of the matrix and the parameter. Dot will panic with ErrShape if
// the column dimension of the receiver does not equal the row dimension of the parameter.
func (d *Dense) Dot(b, c Matrix) Matrix {
	cc, _ := c.(*Dense)
	switch b := b.(type) {
	case *Dense:
		return d.DotDense(b, cc)
	case *Sparse:
		return d.dotSparse(b, cc)
	case *Pivot:
		return d.DotPivot(b, cc)
	default:
		panic("not implemented")
	}

	panic("cannot reach")
}

// DotDense returns the matrix product of the matrix and the parameter as a dense matrix. DotDense will panic
// with ErrShape if the column dimension of the receiver does not equal the row dimension of the parameter.
func (d *Dense) DotDense(b, c *Dense) *Dense {
	if d.cols != b.rows {
		panic(ErrShape)
	}

	if c == b || c == d {
		c = nil
	}
	c = c.reallocate(d.rows, b.cols)

	blasEngine.Dgemm(blas.RowMajor, blas.NoTrans, blas.NoTrans,
		d.rows, b.cols, d.cols,
		1.,
		d.matrix, d.cols,
		b.matrix, b.cols,
		0.,
		c.matrix, c.cols,
	)

	// Pure Go implementation replaces call to blas above, with 1.5-3.2 fold time cost increase.
	// t := make([]float64, b.rows)
	// for i := 0; i < b.cols; i++ {
	// 	var nonZero bool
	// 	for j := 0; j < b.rows; j++ {
	// 		v := b.at(j, i)
	// 		if v != 0 {
	// 			nonZero = true
	// 		}
	// 		t[j] = v
	// 	}
	// 	if nonZero {
	// 		for j := 0; j < d.rows; j++ {
	// 			row := d.matrix[j*d.cols : (j+1)*d.cols]
	// 			c.set(j, i, row.foldMulSum(t))
	// 		}
	// 	}
	// }

	return c
}

func (d *Dense) dotSparse(b *Sparse, c *Dense) *Dense {
	if d.cols != b.rows {
		panic(ErrShape)
	}

	if c == d {
		c = nil
	}
	c = c.reallocate(d.rows, b.cols)

	t := make([]float64, b.rows)
	for i := 0; i < b.cols; i++ {
		var nonZero bool
		for j, row := range b.matrix {
			v := row.at(i)
			if v != 0 {
				nonZero = true
			}
			t[j] = v
		}
		if nonZero {
			for j := 0; j < d.rows; j++ {
				row := d.matrix[j*d.cols : (j+1)*d.cols]
				c.set(j, i, row.foldMulSum(t))
			}
		}
	}

	return c
}

// swap columns of a dense matrix
func (d *Dense) DotPivot(b *Pivot, c *Dense) *Dense {
	if d.cols != len(b.matrix) {
		panic(ErrShape)
	}

	if c != d {
		c = c.reallocate(d.rows, d.cols)
		for to, from := range b.xirtam {
			blasEngine.Dcopy(d.rows, d.matrix[from:], d.cols, c.matrix[to:], c.cols)
		}
		return c
	}

	visit := make([]bool, len(b.xirtam))
	for to, from := range b.xirtam {
		for to != from && !visit[from] {
			visit[from] = true
			blasEngine.Dswap(d.rows, d.matrix[from:], d.cols, c.matrix[to:], c.cols)
			from = b.xirtam[from]
		}
		visit[from] = true
	}

	return c
}

// Augment returns the augmentation of the receiver with the parameter. Augment will panic with
// ErrColLength if the column dimensions of the two matrices do not match.
func (d *Dense) Augment(b, c Matrix) Matrix {
	cc, _ := c.(*Dense)
	switch b := b.(type) {
	case *Dense:
		return d.AugmentDense(b, cc)
	case *Sparse:
		return d.augmentSparse(b, cc)
	case *Pivot:
		return d.augmentPivot(b, cc)
	default:
		panic("not implemented")
	}

	panic("cannot reach")
}

// AugmentDense returns the augmentation of the receiver with the parameter as a dense matrix.
// AugmentDense will panic with ErrColLength if the column dimensions of the two matrices do not match.
func (d *Dense) AugmentDense(b, c *Dense) *Dense {
	if d.rows != b.rows {
		panic(ErrColLength)
	}

	c = c.reallocate(d.rows, d.cols+b.cols)
	for i := 0; i < c.rows; i++ {
		copy(c.matrix[i*c.cols:i*c.cols+d.cols], d.matrix[i*d.cols:(i+1)*d.cols])
		copy(c.matrix[i*c.cols+d.cols:(i+1)*c.cols], b.matrix[i*b.cols:(i+1)*b.cols])
	}

	return c
}

func (d *Dense) augmentSparse(b *Sparse, c *Dense) *Dense {
	if d.rows != b.rows {
		panic(ErrColLength)
	}

	c = c.reallocate(d.rows, d.cols+b.cols)
	c.matrix.zero()
	for i := 0; i < c.rows; i++ {
		copy(c.matrix[i*c.cols:i*c.cols+d.cols], d.matrix[i*d.cols:(i+1)*d.cols])
		for _, e := range b.matrix[i] {
			c.set(i, d.cols+e.index, e.value)
		}
	}

	return c
}

func (d *Dense) augmentPivot(b *Pivot, c *Dense) *Dense {
	if d.rows != len(b.matrix) {
		panic(ErrColLength)
	}

	c = c.reallocate(d.rows, d.cols+len(b.matrix))
	c.matrix.zero()
	for i := 0; i < c.rows; i++ {
		copy(c.matrix[i*c.cols:i*c.cols+d.cols], d.matrix[i*d.cols:(i+1)*d.cols])
	}
	for i, j := range b.xirtam {
		c.set(i, d.cols+j, 1)
	}

	return c
}

// Stack returns the stacking of the receiver with the parameter. Stack will panic with
// ErrRowLength if the column dimensions of the two matrices do not match.
func (d *Dense) Stack(b, c Matrix) Matrix {
	cc, _ := c.(*Dense)
	switch b := b.(type) {
	case *Dense:
		return d.StackDense(b, cc)
	case *Sparse:
		return d.stackSparse(b, cc)
	case *Pivot:
		return d.stackPivot(b, cc)
	default:
		panic("not implemented")
	}

	panic("cannot reach")
}

// StackDense returns the augmentation of the receiver with the parameter as a dense matrix.
// StackDense will panic with ErrRowLength if the column dimensions of the two matrices do not match.
func (d *Dense) StackDense(b, c *Dense) *Dense {
	if d.cols != b.cols {
		panic(ErrRowLength)
	}

	c = c.reallocate(d.rows+b.rows, d.cols)
	copy(c.matrix, d.matrix)
	copy(c.matrix[len(d.matrix):], b.matrix)

	return c
}

func (d *Dense) stackSparse(b *Sparse, c *Dense) *Dense {
	if d.cols != b.cols {
		panic(ErrRowLength)
	}

	c = c.reallocate(d.rows+b.rows, d.cols)
	copy(c.matrix, d.matrix)
	c.matrix[d.rows*d.cols:].zero()
	for i, row := range b.matrix {
		for _, e := range row {
			c.set(d.rows+i, e.index, e.value)
		}
	}

	return c
}

func (d *Dense) stackPivot(b *Pivot, c *Dense) *Dense {
	if d.cols != len(b.matrix) {
		panic(ErrColLength)
	}

	c = c.reallocate(d.rows+len(b.matrix), d.cols)
	copy(c.matrix, d.matrix)
	c.matrix[d.rows*d.cols:].zero()
	for i, j := range b.xirtam {
		c.set(d.rows+i, j, 1)
	}

	return c
}

// Filter return a matrix with all elements at (r, c) set to zero where FilterFunc(r, c, v) returns false.
func (d *Dense) Filter(f FilterFunc, c Matrix) Matrix {
	cc, _ := c.(*Dense)
	return d.FilterDense(f, cc)
}

// FilterDense return a dense matrix with all elements at (r, c) set to zero where FilterFunc(r, c, v) returns false.
func (d *Dense) FilterDense(f FilterFunc, c *Dense) *Dense {
	if c == d {
		for i, e := range d.matrix {
			if !f(i/d.cols, i%d.cols, e) {
				c.matrix[i] = 0
			}
		}
		return c
	}
	c = c.reallocate(d.Dims())
	c.matrix.zero()
	for i, e := range d.matrix {
		if f(i/d.cols, i%d.cols, e) {
			c.matrix[i] = e
		}
	}

	return c
}

// Apply returns a matrix which has had a function applied to all elements of the matrix.
func (d *Dense) Apply(f ApplyFunc, c Matrix) Matrix {
	cc, _ := c.(*Dense)
	return d.ApplyDense(f, cc)
}

// ApplyDense returns a dense matrix which has had a function applied to all elements of the matrix.
func (d *Dense) ApplyDense(f ApplyFunc, c *Dense) *Dense {
	if c != d {
		c = c.reallocate(d.Dims())
	}
	for i, e := range d.matrix {
		c.matrix[i] = f(i/d.cols, i%d.cols, e)
	}

	return c
}

// ApplyAll returns a matrix which has had a function applied to all elements of the matrix.
func (d *Dense) ApplyAll(f ApplyFunc, c Matrix) Matrix {
	cc, _ := c.(*Dense)
	return d.ApplyDense(f, cc)
}

// ApplyAllDense returns a matrix which has had a function applied to all elements of the matrix.
func (d *Dense) ApplyAllDense(f ApplyFunc, c *Dense) Matrix { return d.ApplyDense(f, c) }

// Format satisfies the fmt.Formatter interface.
func (d *Dense) Format(fs fmt.State, c rune) {
	if c == 'v' && fs.Flag('#') {
		fmt.Fprintf(fs, "&%#v", *d)
		return
	}
	Format(d, d.Margin, '.', fs, c)
}
