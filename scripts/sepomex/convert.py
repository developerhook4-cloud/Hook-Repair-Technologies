#!/usr/bin/env python3
"""Regenera db/data/sepomex.tsv.gz a partir del catálogo oficial de SEPOMEX.

Uso:
    python3 scripts/sepomex/convert.py CPdescargatxt.zip db/data/sepomex.tsv.gz

La entrada puede ser el .zip (CPdescargatxt.zip) o el .txt (CPdescarga.txt)
que se baja de Correos de México:
https://www.correosdemexico.gob.mx/SSLServicios/ConsultaCP/CodigoPostal_Exportar.aspx
(estado "Todos", formato "txt"). El archivo original es texto separado por "|"
en latin1; la salida es TSV en UTF-8, comprimido, sin encabezado, con las
columnas: cp, colonia, tipo, municipio, ciudad, estado (en ese orden, que es el
que lee db.LoadPostalCatalog). La salida es determinista (ordenada y con mtime 0
en el gzip) para que regenerarla con los mismos datos no cambie el archivo.
"""
import gzip
import io
import sys
import zipfile

# Columnas del catálogo original usadas aquí.
CAMPOS = ["d_codigo", "d_asenta", "d_tipo_asenta", "D_mnpio", "d_ciudad", "d_estado"]


def leer(ruta):
    if ruta.lower().endswith(".zip"):
        with zipfile.ZipFile(ruta) as z:
            nombre = next(n for n in z.namelist() if n.lower().endswith(".txt"))
            return z.read(nombre).decode("latin1")
    with open(ruta, "rb") as f:
        return f.read().decode("latin1")


def convertir(texto):
    lineas = texto.splitlines()
    # La primera línea es un aviso legal; el encabezado es la que empieza con d_codigo.
    i = next(n for n, l in enumerate(lineas) if l.startswith("d_codigo|"))
    cols = lineas[i].split("|")
    pos = [cols.index(c) for c in CAMPOS]
    filas = set()
    for l in lineas[i + 1:]:
        if not l.strip():
            continue
        f = l.split("|")
        fila = tuple(f[p].strip().replace("\t", " ") for p in pos)
        if len(fila[0]) != 5 or not fila[0].isdigit() or not fila[1]:
            raise SystemExit("fila inválida: %r" % l)
        filas.add(fila)
    return sorted(filas)


def main():
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    filas = convertir(leer(sys.argv[1]))
    buf = io.BytesIO()
    with gzip.GzipFile(fileobj=buf, mode="wb", compresslevel=9, mtime=0) as gz:
        gz.write(("\n".join("\t".join(f) for f in filas) + "\n").encode("utf-8"))
    with open(sys.argv[2], "wb") as out:
        out.write(buf.getvalue())
    print("%d filas, %d bytes" % (len(filas), len(buf.getvalue())))


if __name__ == "__main__":
    main()
