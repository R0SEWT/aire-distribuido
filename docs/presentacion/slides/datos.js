// Generado por datos.py desde reports/*.json. No editar a mano.
window.DATOS = {
 "embudo": {
  "leidas": 2883877,
  "procesadas": 1434498,
  "descartadas": 1449379,
  "motivos": [
   {
    "motivo": "día entero en 0 (no medido)",
    "filas": 762744
   },
   {
    "motivo": "fila sin valor",
    "filas": 262743
   },
   {
    "motivo": "promedio diario o móvil",
    "filas": 218998
   },
   {
    "motivo": "gas repetido en otra unidad",
    "filas": 134285
   },
   {
    "motivo": "fuera del rango físico",
    "filas": 51219
   },
   {
    "motivo": "otras reglas",
    "filas": 19390
   }
  ],
  "hora_inferida": 1333932,
  "anchas": 236001,
  "completas": 65873,
  "estaciones": 33,
  "estaciones_gold": 17
 },
 "speedup": {
  "cargador": [
   {
    "w": 1,
    "punto": 1,
    "inf": 0.9516830878953798,
    "sup": 1.0494566319863516
   },
   {
    "w": 2,
    "punto": 1.7959738806121053,
    "inf": 1.7047630828584286,
    "sup": 1.9093066090858741
   },
   {
    "w": 4,
    "punto": 2.8325549758496877,
    "inf": 2.7362985198448317,
    "sup": 2.9771772536510617
   },
   {
    "w": 8,
    "punto": 4.172403465602901,
    "inf": 3.994408653359615,
    "sup": 4.4027246282793735
   },
   {
    "w": 12,
    "punto": 4.937271161830521,
    "inf": 4.6627245012966805,
    "sup": 5.213310063387911
   }
  ],
  "kmeans_1000": [
   {
    "w": 1,
    "punto": 1.0191755673999483,
    "inf": 0.8531526829087601,
    "sup": 1.22385076283952
   },
   {
    "w": 2,
    "punto": 1.438046679906273,
    "inf": 1.2243015308527585,
    "sup": 1.7112316193171397
   },
   {
    "w": 4,
    "punto": 2.565120219100234,
    "inf": 2.2127845166482167,
    "sup": 3.047604417768399
   },
   {
    "w": 8,
    "punto": 3.1922187396073136,
    "inf": 2.7533568347340345,
    "sup": 3.8010155224298936
   },
   {
    "w": 12,
    "punto": 3.856813254648439,
    "inf": 3.279215114420165,
    "sup": 4.59661480863827
   }
  ],
  "kmeans_10000": [
   {
    "w": 1,
    "punto": 0.9255324005262903,
    "inf": 0.7516380780581204,
    "sup": 1.132550200354777
   },
   {
    "w": 2,
    "punto": 1.4781996296098918,
    "inf": 1.2609385423647648,
    "sup": 1.7661027312583988
   },
   {
    "w": 4,
    "punto": 2.0990213978679657,
    "inf": 1.7489177658601793,
    "sup": 2.538311914174836
   },
   {
    "w": 8,
    "punto": 2.4272860071934224,
    "inf": 2.109693136376313,
    "sup": 2.870932538058117
   },
   {
    "w": 12,
    "punto": 2.250513394011946,
    "inf": 1.6719428717775664,
    "sup": 3.0104716386929025
   }
  ],
  "chunks_1000": 66,
  "chunks_10000": 7
 },
 "niveles": [
  {
   "nivel": "bajo",
   "regimen": 0,
   "horas": 48161,
   "fraccion": 0.731118971353969,
   "pm2_5": 9.037549677124652,
   "pm10": 17.63909927119451,
   "eca": "no_excede",
   "por_hora": [
    2415,
    2406,
    2381,
    2330,
    2245,
    2149,
    2013,
    1645,
    1587,
    1757,
    1671,
    1633,
    1705,
    1772,
    1815,
    1709,
    1798,
    1966,
    2008,
    1992,
    2159,
    2296,
    2347,
    2362
   ],
   "estaciones": [
    {
     "estacion": "CA-CP-01",
     "horas": 12393,
     "fraccion": 0.25732439110483585,
     "departamento": "Cusco",
     "altitud": "3600"
    },
    {
     "estacion": "CA-MAC-01",
     "horas": 7495,
     "fraccion": 0.15562384501982932,
     "departamento": "Piura",
     "altitud": "38"
    },
    {
     "estacion": "CA-HUA-01",
     "horas": 6623,
     "fraccion": 0.13751790868129815,
     "departamento": "Piura",
     "altitud": "32"
    },
    {
     "estacion": "CA-AYAS-01",
     "horas": 6328,
     "fraccion": 0.13139262058512075,
     "departamento": "Ancash",
     "altitud": "3728"
    },
    {
     "estacion": "CA-JUPR-01",
     "horas": 5269,
     "fraccion": 0.10940387450426693,
     "departamento": "Ancash",
     "altitud": "4138"
    }
   ]
  },
  {
   "nivel": "medio",
   "regimen": 2,
   "horas": 16176,
   "fraccion": 0.2455634326658874,
   "pm2_5": 24.381915182987196,
   "pm10": 58.22880069238359,
   "eca": "no_excede",
   "por_hora": [
    311,
    320,
    339,
    375,
    453,
    534,
    647,
    962,
    1016,
    890,
    937,
    987,
    938,
    889,
    852,
    918,
    897,
    734,
    699,
    728,
    569,
    435,
    382,
    364
   ],
   "estaciones": [
    {
     "estacion": "CA-CP-01",
     "horas": 3753,
     "fraccion": 0.23201038575667657,
     "departamento": "Cusco",
     "altitud": "3600"
    },
    {
     "estacion": "CA-ROM-01",
     "horas": 3752,
     "fraccion": 0.23194856577645895,
     "departamento": "La Libertad",
     "altitud": "177"
    },
    {
     "estacion": "CA-CG-02",
     "horas": 3194,
     "fraccion": 0.19745301681503463,
     "departamento": "La Libertad",
     "altitud": "137"
    },
    {
     "estacion": "CA-AYAS-01",
     "horas": 997,
     "fraccion": 0.061634520276953514,
     "departamento": "Ancash",
     "altitud": "3728"
    },
    {
     "estacion": "CA-HUA-01",
     "horas": 810,
     "fraccion": 0.05007418397626113,
     "departamento": "Piura",
     "altitud": "32"
    }
   ]
  },
  {
   "nivel": "alto",
   "regimen": 1,
   "horas": 1536,
   "fraccion": 0.02331759598014361,
   "pm2_5": 65.12396484374996,
   "pm10": 186.6618229166667,
   "eca": "excede",
   "por_hora": [
    28,
    29,
    33,
    43,
    51,
    59,
    81,
    138,
    144,
    98,
    125,
    103,
    77,
    66,
    66,
    110,
    47,
    47,
    45,
    33,
    29,
    27,
    29,
    28
   ],
   "estaciones": [
    {
     "estacion": "CA-CP-01",
     "horas": 543,
     "fraccion": 0.353515625,
     "departamento": "Cusco",
     "altitud": "3600"
    },
    {
     "estacion": "CA-HU-04",
     "horas": 219,
     "fraccion": 0.142578125,
     "departamento": "Lima",
     "altitud": "360"
    },
    {
     "estacion": "CA-RZC-02",
     "horas": 163,
     "fraccion": 0.10611979166666667,
     "departamento": "Lima",
     "altitud": "504"
    },
    {
     "estacion": "CA-CG-02",
     "horas": 122,
     "fraccion": 0.07942708333333333,
     "departamento": "La Libertad",
     "altitud": "137"
    },
    {
     "estacion": "CA-ESP-01",
     "horas": 114,
     "fraccion": 0.07421875,
     "departamento": "Cusco",
     "altitud": "3926"
    }
   ]
  }
 ],
 "eca": {
  "pm2_5": 50,
  "pm10": 100
 },
 "convencion_hora": "fin",
 "pruebas": 240
};
