// Package kmeans implementa K-Means (Lloyd) sobre las lecturas de calidad del
// aire, en versión secuencial y concurrente, usando solo la biblioteca estándar.
//
// Portado del Trabajo Parcial del mismo equipo, upc.edu.pe/concurrente/tp/kmeans.
// El contrato numérico se conserva tal cual (tp/docs/kmeans.md en el repo del curso):
// distancia euclídea al cuadrado, desempate por el centroide de menor índice,
// k-means++ con semilla fija ejecutado una vez, inercia reportada con los
// centroides finales, clusters vacíos que conservan su centroide y reducción de
// los parciales en orden de chunk.
package kmeans
