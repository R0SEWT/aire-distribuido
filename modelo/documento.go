package modelo

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/R0SEWT/aire-distribuido/almacen"
)

// Documento devuelve el modelo como se guarda en la colección modelos de
// MongoDB y en la clave modelo:activo de Redis (spec §7): con todo lo que
// DesdeDocumento necesita para volver a armarlo. Activo queda en false: lo
// decide quien lo guarda.
func (m *Modelo) Documento() almacen.Modelo {
	k, d := m.K(), len(m.Receta.Features)
	doc := almacen.Modelo{
		ID:              m.ID,
		Creado:          m.Creado,
		Origen:          m.Origen,
		Receta:          m.Receta,
		GoldSHA256:      m.GoldSHA256,
		Estandarizacion: *m.est,
		CentroidesStd:   make([][]float64, k),
		Regimenes:       make([]almacen.RegimenModelo, k),
		Inercia:         m.Inercia,
		Iteraciones:     m.Iteraciones,
	}
	for j := range k {
		doc.CentroidesStd[j] = slices.Clone(m.centroides[j*d : (j+1)*d])
		doc.Regimenes[j] = almacen.RegimenModelo(m.Regimenes[j])
	}
	for _, c := range m.CentroidesOriginales {
		doc.Centroides = append(doc.Centroides, slices.Clone(c))
	}
	return doc
}

// DesdeDocumento arma un modelo a partir de su documento: es como la API
// recupera de Redis el modelo activo al arrancar. Comprueba lo mismo que
// Sembrar (receta válida, features que salen de una lectura, estandarización de
// esa receta, k centroides finitos de la dimensión correcta, un régimen por
// centroide) porque el documento viene de fuera del proceso. Los centroides en
// unidades originales no se leen del documento: se recalculan.
func DesdeDocumento(doc almacen.Modelo) (*Modelo, error) {
	m, err := desdeDocumento(doc)
	if err != nil {
		return nil, fmt.Errorf("modelo %q: %w", doc.ID, err)
	}
	return m, nil
}

func desdeDocumento(doc almacen.Modelo) (*Modelo, error) {
	if doc.ID == "" {
		return nil, errors.New("el documento no tiene id")
	}
	rec := doc.Receta
	if err := rec.Validar(); err != nil {
		return nil, err
	}
	for _, f := range rec.Features {
		if !slices.Contains(derivables, f) {
			return nil, fmt.Errorf("usa la feature %q, que no sale de una lectura de pm2_5 y pm10", f)
		}
	}
	est := doc.Estandarizacion
	if err := est.Validar(); err != nil {
		return nil, err
	}
	if err := est.Coincide(rec); err != nil {
		return nil, err
	}
	k, d := len(doc.CentroidesStd), len(rec.Features)
	if k == 0 || k != rec.K || len(doc.Regimenes) != k {
		return nil, fmt.Errorf("k no coincide: la receta dice %d, hay %d centroides y %d regímenes", rec.K, k, len(doc.Regimenes))
	}
	m := &Modelo{
		ID:          doc.ID,
		Origen:      doc.Origen,
		Creado:      doc.Creado,
		GoldSHA256:  doc.GoldSHA256,
		Receta:      rec,
		Iteraciones: doc.Iteraciones,
		Inercia:     doc.Inercia,
		Regimenes:   make([]Regimen, k),
		centroides:  make([]float64, 0, k*d),
		est:         &est,
	}
	for j, c := range doc.CentroidesStd {
		if len(c) != d {
			return nil, fmt.Errorf("el centroide %d tiene %d valores y hay %d features", j, len(c), d)
		}
		for _, v := range c {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("el centroide %d no es finito: %v", j, c)
			}
		}
		m.centroides = append(m.centroides, c...)
	}
	for j, r := range doc.Regimenes {
		if r.Regimen != j {
			return nil, fmt.Errorf("en la posición %d está el régimen %d", j, r.Regimen)
		}
		m.Regimenes[j] = Regimen(r)
	}
	if est.CentroidesEnUnidades() {
		m.CentroidesOriginales = make([][]float64, k)
		for j := range k {
			m.CentroidesOriginales[j] = make([]float64, d)
			if err := est.Invertir(m.centroides[j*d:(j+1)*d], "", m.CentroidesOriginales[j]); err != nil {
				return nil, err
			}
		}
	}
	return m, nil
}
