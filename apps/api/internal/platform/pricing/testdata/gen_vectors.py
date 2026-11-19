#!/usr/bin/env python3
"""Oráculo independiente de pricing.Compute: genera vectors.json.

Se escribió desde cero a partir de la especificación en prosa
`pricing-spec.md` (la misma que describen el CHANGELOG y el PR de
pricing.Compute), del ADR 0008 y del §2 de data-model.md, sin ver la
implementación en Go ni ningún generador anterior. Por eso sirve para
verificarla: si Go y este oráculo coinciden, coinciden dos lecturas
independientes de la misma especificación.

Uso: python3 -I gen_vectors.py vectors.json

Solo usa la biblioteca estándar, decimal.Decimal con ROUND_HALF_UP en un
contexto propio y una semilla fija: dos corridas dan el mismo archivo.
"""

import json
import random
import sys
from decimal import ROUND_FLOOR, ROUND_HALF_UP, Context, Decimal, localcontext

# Contexto propio: precisión de sobra para 1000 líneas de 14 dígitos por
# 10 decimales internos, y half-up (la mitad se aleja de cero).
CTX = Context(prec=80, rounding=ROUND_HALF_UP)

SEED = 20261119

IGV_DIVISOR = Decimal("1.18")
IGV_RATE = Decimal("0.18")
DRIFT_LIMIT = Decimal("0.50")
MAX_AMOUNT = Decimal("999999999999.99")
MAX_QTY_PRICE = Decimal("9999999999.9999")
MAX_RATE = Decimal("999.999")
MAX_LINES = 1000
ZERO = Decimal("0")

AFECTACIONES = ("gravado", "exonerado", "inafecto", "gratuito")
TOTAL_KEYS = (
    "gravado", "exonerado", "inafecto", "gratuito", "subtotal",
    "discounts", "igv", "freeIgv", "otherCharges", "total",
)
# Orden del §2.d para el control de desborde de los totales.
OVERFLOW_KEYS = (
    "gravado", "exonerado", "inafecto", "gratuito", "subtotal",
    "discounts", "igv", "freeIgv", "total",
)


class OracleError(Exception):
    """Error de validación con su código y campo (§2)."""

    def __init__(self, code, field):
        super().__init__(f"{code} at {field}")
        self.code = code
        self.field = field


def fail(field, code="validation_failed"):
    raise OracleError(code, field)


# --- utilidades decimales ----------------------------------------------------

def rnd(x, places):
    """Redondea half-up a `places` decimales y quita el cero negativo."""
    q = x.quantize(Decimal(1).scaleb(-places), rounding=ROUND_HALF_UP, context=CTX)
    if q == 0:
        q = abs(q)
    return q


def fmt(x, places):
    """Escribe con escala fija; el valor ya tiene que estar en esa escala."""
    q = rnd(x, places)
    assert q == x, f"value {x} is not exact at scale {places}"
    assert q >= 0, f"negative amount {q}"
    return f"{q:.{places}f}"


def decimals_of(d):
    """Escala del valor, no dígitos escritos: los ceros finales no cuentan
    ("1.00000" vale como 1, §1)."""
    if d == 0:
        return 0
    exp = d.normalize(context=CTX).as_tuple().exponent
    return -exp if exp < 0 else 0


def parse(s):
    return Decimal(s)


# --- el oráculo --------------------------------------------------------------

def validate_input(inp):
    """§2.a, §2.b y §2.c: comprobaciones previas al cálculo, en orden."""
    currency = inp.get("currency")
    if currency not in ("PEN", "USD"):
        fail("currency")
    has_rate = "exchangeRate" in inp
    if currency == "PEN" and has_rate:
        fail("exchangeRate")
    if currency == "USD":
        if not has_rate:
            fail("exchangeRate", "exchange_rate_required")
        rate = parse(inp["exchangeRate"])
        if not (rate > 0) or rate > MAX_RATE or decimals_of(rate) > 3:
            fail("exchangeRate")

    lines = inp["lines"]
    if not (1 <= len(lines) <= MAX_LINES):
        fail("lines")
    for i, line in enumerate(lines):
        prefix = f"lines[{i}]."
        qty = parse(line["quantity"])
        if not (qty > 0) or qty > MAX_QTY_PRICE or decimals_of(qty) > 4:
            fail(prefix + "quantity")
        price = parse(line["unitPrice"])
        if price < 0 or price > MAX_QTY_PRICE or decimals_of(price) > 4:
            fail(prefix + "unitPrice")
        disc = parse(line["discount"])
        if disc < 0 or decimals_of(disc) > 2:
            fail(prefix + "discount")
        gross = CTX.multiply(qty, price)
        if gross > MAX_AMOUNT:
            fail(prefix + "unitPrice")
        if disc > gross:
            fail(prefix + "discount")
        if line["afectacion"] not in AFECTACIONES:
            fail(prefix + "afectacion")

    for field in ("globalDiscount", "otherCharges"):
        v = parse(inp[field])
        if v < 0 or decimals_of(v) > 2 or v > MAX_AMOUNT:
            fail(field)


def line_values(line):
    """§3: PT, VU, RB y RI de una línea (RB/RI solo en gravado y gratuito)."""
    qty = parse(line["quantity"])
    price = parse(line["unitPrice"])
    disc = parse(line["discount"])
    aff = line["afectacion"]
    pt = rnd(CTX.subtract(CTX.multiply(qty, price), disc), 2)
    vu = rnd(CTX.divide(price, IGV_DIVISOR), 10)
    rb = ri = None
    if aff in ("gravado", "gratuito"):
        disc_base = rnd(CTX.divide(disc, IGV_DIVISOR), 10)
        rb = rnd(CTX.subtract(CTX.multiply(qty, vu), disc_base), 2)
        if rb <= 0:
            rb = ZERO
        ri = rnd(CTX.multiply(rb, IGV_RATE), 2)
    # En exonerado e inafecto el precio no lleva IGV: el valor unitario es el precio.
    if aff in ("gravado", "gratuito"):
        unit_value = rnd(CTX.divide(price, IGV_DIVISOR), 4)
    else:
        unit_value = rnd(price, 4)
    return {"aff": aff, "pt": pt, "rb": rb, "ri": ri, "unitValue": unit_value}


def allocate(d_cents, group_cents):
    """§5: reparte D en céntimos entre gravado, exonerado e inafecto."""
    total = sum(group_cents)
    if d_cents == 0 or total == 0:
        return [0, 0, 0]
    shares = []
    remainders = []
    for g in group_cents:
        shares.append(d_cents * g // total)
        remainders.append(d_cents * g % total)
    leftover = d_cents - sum(shares)
    # Mayor resto primero; en empate, el orden fijo gravado, exonerado, inafecto.
    order = sorted(
        (k for k in range(3) if group_cents[k] > 0),
        key=lambda k: (-remainders[k], k),
    )
    assert leftover <= len(order)
    for k in order[:leftover]:
        shares[k] += 1
    assert sum(shares) == d_cents
    return shares


def cents(x):
    c = CTX.multiply(x, Decimal(100))
    assert c == c.to_integral_value(), x
    return int(c)


def compute_raw(inp):
    """Cálculo completo (§3-§6) sin el control posterior del §2.d."""
    vals = [line_values(l) for l in inp["lines"]]

    drift = ZERO
    for v in vals:
        if v["aff"] == "gravado":
            drift += abs(v["pt"] - (v["rb"] + v["ri"]))
    mode = "base_rate" if drift > DRIFT_LIMIT else "price"

    out_lines = []
    for v in vals:
        aff, pt = v["aff"], v["pt"]
        if aff in ("gravado", "gratuito"):
            if mode == "price":
                base = rnd(CTX.divide(pt, IGV_DIVISOR), 2)
                igv = pt - base
            else:
                base, igv = v["rb"], v["ri"]
            total = (base + igv) if aff == "gravado" else ZERO
            if aff == "gravado" and mode == "price":
                assert total == pt
        else:
            base, igv, total = pt, ZERO, pt
        out_lines.append({"aff": aff, "unitValue": v["unitValue"],
                          "base": base, "igv": igv, "total": total})

    # §4: sumas de valores ya redondeados.
    B = sum((l["base"] for l in out_lines if l["aff"] == "gravado"), ZERO)
    V = sum((l["igv"] for l in out_lines if l["aff"] == "gravado"), ZERO)
    G = sum((l["total"] for l in out_lines if l["aff"] == "gravado"), ZERO)
    E = sum((l["total"] for l in out_lines if l["aff"] == "exonerado"), ZERO)
    I = sum((l["total"] for l in out_lines if l["aff"] == "inafecto"), ZERO)
    FB = sum((l["base"] for l in out_lines if l["aff"] == "gratuito"), ZERO)
    FV = sum((l["igv"] for l in out_lines if l["aff"] == "gratuito"), ZERO)
    assert G == B + V

    D = parse(inp["globalDiscount"])
    other = parse(inp["otherCharges"])
    cap = G + E + I
    if D > cap:
        return {"capError": True}

    s_g, s_e, s_i = (Decimal(c) / 100 for c in allocate(cents(D), [cents(G), cents(E), cents(I)]))
    assert s_g + s_e + s_i == D

    # §5, grupo gravado.
    if s_g == 0:
        base_after, igv_after = B, V
    else:
        total_after = G - s_g
        cut_base = rnd(CTX.divide(s_g, IGV_DIVISOR), 2)
        igv_after = V - (s_g - cut_base)
        if igv_after < 0:
            igv_after = ZERO
        if igv_after > total_after:
            igv_after = total_after
        base_after = total_after - igv_after
    assert ZERO <= base_after <= B
    assert ZERO <= igv_after <= V

    subtotal = B + E + I
    discounts = (B - base_after) + s_e + s_i
    total = subtotal - discounts + igv_after + other
    totals = {
        "gravado": B, "exonerado": E, "inafecto": I, "gratuito": FB,
        "subtotal": subtotal, "discounts": discounts, "igv": igv_after,
        "freeIgv": FV, "otherCharges": other, "total": total,
    }
    return {"capError": False, "mode": mode, "lines": out_lines, "totals": totals}


def compute(inp, check_invariants=True):
    """Devuelve ('want', resultado) o ('error', {code, field})."""
    try:
        validate_input(inp)
        res = compute_raw(inp)
        if res["capError"]:
            fail("globalDiscount")
        for key in OVERFLOW_KEYS:
            if res["totals"][key] > MAX_AMOUNT:
                fail("totals." + key)
    except OracleError as e:
        return "error", {"code": e.code, "field": e.field}
    if check_invariants:
        assert_invariants(inp, res)
    return "want", res


def assert_invariants(inp, res):
    """§6: invariantes que siempre se cumplen."""
    t = res["totals"]
    D = parse(inp["globalDiscount"])
    # Identidad del total.
    assert t["total"] == t["subtotal"] - t["discounts"] + t["igv"] + t["otherCharges"]
    assert t["discounts"] >= 0
    for key in TOTAL_KEYS:
        assert ZERO <= t[key] <= MAX_AMOUNT, (key, t[key])
    # Mismo comprobante sin descuento global.
    plain = dict(inp)
    plain["globalDiscount"] = "0"
    kind, base_res = compute(plain, check_invariants=False)
    assert kind == "want"
    bt = base_res["totals"]
    assert bt["discounts"] == 0
    # El total baja exactamente D.
    assert bt["total"] - t["total"] == D, (bt["total"], t["total"], D)
    # El IGV y el total no crecen con el descuento.
    assert t["igv"] <= bt["igv"]
    assert t["total"] <= bt["total"]
    assert res["mode"] == base_res["mode"]


def render_want(res):
    return {
        "mode": res["mode"],
        "lines": [
            {
                "unitValue": fmt(l["unitValue"], 4),
                "base": fmt(l["base"], 2),
                "igv": fmt(l["igv"], 2),
                "total": fmt(l["total"], 2),
            }
            for l in res["lines"]
        ],
        "totals": {k: fmt(res["totals"][k], 2) for k in TOTAL_KEYS},
    }


# --- construcción de vectores ------------------------------------------------

def line(quantity, unit_price, discount="0", afectacion="gravado"):
    return {"quantity": quantity, "unitPrice": unit_price,
            "discount": discount, "afectacion": afectacion}


def make_input(lines, global_discount="0", other_charges="0", currency="PEN",
               exchange_rate=None):
    inp = {"currency": currency}
    if exchange_rate is not None:
        inp["exchangeRate"] = exchange_rate
    inp["lines"] = lines
    inp["globalDiscount"] = global_discount
    inp["otherCharges"] = other_charges
    return inp


def dec_str(units, scale):
    """Convierte un entero de unidades mínimas a cadena con `scale` decimales,
    sin ceros finales sobrantes (salvo los que aporta la escala elegida)."""
    d = Decimal(units).scaleb(-scale)
    s = f"{d:.{scale}f}"
    return s


class Builder:
    def __init__(self):
        self.vectors = []
        self.names = set()
        self.categories = {}

    def add(self, category, name, inp, expect_error=None, expect_mode=None):
        full = f"{category}/{name}"
        assert full not in self.names, full
        self.names.add(full)
        kind, res = compute(inp)
        vec = {"name": full, "input": inp}
        if kind == "error":
            # La etiqueta la calcula el oráculo; esto solo comprueba que el
            # vector ejercita la rama que se quiso construir.
            if expect_error is not None:
                assert res["field"] == expect_error[1] and res["code"] == expect_error[0], (full, res)
            vec["error"] = res
        else:
            assert expect_error is None, (full, "expected error", expect_error)
            if expect_mode is not None:
                assert res["mode"] == expect_mode, (full, res["mode"])
            vec["want"] = render_want(res)
        self.vectors.append(vec)
        self.categories[category] = self.categories.get(category, 0) + 1
        return kind, res


CHEAP_PRICES = ("0.10", "0.16", "0.43")


def rand_price(rng):
    pick = rng.random()
    if pick < 0.25:
        return rng.choice(CHEAP_PRICES + ("0.01", "0.05", "0.99", "1.50", "3.50"))
    if pick < 0.55:
        return dec_str(rng.randint(1, 50000), 2)
    if pick < 0.75:
        return str(rng.randint(1, 2000))
    if pick < 0.9:
        return dec_str(rng.randint(1, 9999999), 4)
    return dec_str(rng.randint(1, 999), 1)


def rand_qty(rng):
    pick = rng.random()
    if pick < 0.55:
        return str(rng.randint(1, 12))
    if pick < 0.8:
        return dec_str(rng.randint(1, 99999), 4)
    if pick < 0.9:
        return dec_str(rng.randint(1, 999), 2)
    return str(rng.randint(13, 500))


def rand_line(rng, aff=None, allow_discount=True):
    qty = rand_qty(rng)
    price = rand_price(rng)
    gross = Decimal(qty) * Decimal(price)
    disc = "0"
    if allow_discount and rng.random() < 0.3 and gross >= Decimal("0.01"):
        top = int((gross * 100).to_integral_value(rounding=ROUND_FLOOR))
        disc = dec_str(rng.randint(0, top), 2)
    if aff is None:
        aff = rng.choices(AFECTACIONES, weights=(6, 2, 2, 1))[0]
    return line(qty, price, disc, aff)


def cap_of(inp):
    plain = dict(inp)
    plain["globalDiscount"] = "0"
    kind, res = compute(plain, check_invariants=False)
    assert kind == "want"
    t = res["totals"]
    # G + E + I = subtotal + igv sin descuento.
    return t["subtotal"] + t["igv"]


def rand_global(rng, inp, pick_mode=None):
    cap = cap_of(inp)
    cap_c = cents(cap)
    if cap_c == 0:
        return "0"
    r = rng.random() if pick_mode is None else pick_mode
    if r < 0.2:
        return "0"
    if r < 0.4:
        return dec_str(rng.randint(1, min(cap_c, 9)), 2)
    if r < 0.95:
        return dec_str(rng.randint(1, cap_c), 2)
    return dec_str(cap_c, 2)


def build():
    rng = random.Random(SEED)
    b = Builder()

    # Regresión: tres carritos con descuento global de 0.01.
    b.add("regression", "50x1.50-global-0.01",
          make_input([line("1", "1.50") for _ in range(50)], "0.01"))
    b.add("regression", "30x3.50-global-0.01",
          make_input([line("1", "3.50") for _ in range(30)], "0.01"))
    b.add("regression", "200x0.43-global-0.01",
          make_input([line("1", "0.43") for _ in range(200)], "0.01"),
          expect_mode="base_rate")

    # Borde del desvío: 0.10 aporta 0.01 por línea; 50 líneas = 0.50, 51 = 0.51.
    for n, mode in ((49, "price"), (50, "price"), (51, "base_rate"), (52, "base_rate")):
        b.add("drift-boundary", f"{n}x0.10", make_input([line("1", "0.10") for _ in range(n)]),
              expect_mode=mode)
        b.add("drift-boundary", f"{n}x0.10-global-0.01",
              make_input([line("1", "0.10") for _ in range(n)], "0.01"), expect_mode=mode)
    # El mismo borde, pero con líneas exoneradas e inafectas que no suman desvío.
    b.add("drift-boundary", "50x0.10-plus-exo-ina",
          make_input([line("1", "0.10") for _ in range(50)]
                     + [line("1", "0.10", afectacion="exonerado"),
                        line("1", "0.10", afectacion="inafecto")], "0.03"),
          expect_mode="price")
    b.add("drift-boundary", "51x0.10-plus-exo-ina",
          make_input([line("1", "0.10") for _ in range(51)]
                     + [line("1", "0.10", afectacion="exonerado"),
                        line("1", "0.10", afectacion="inafecto")], "0.03"),
          expect_mode="base_rate")
    # Los gratuitos no entran en el desvío.
    b.add("drift-boundary", "50x0.10-plus-60-free",
          make_input([line("1", "0.10") for _ in range(50)]
                     + [line("1", "0.10", afectacion="gratuito") for _ in range(60)]),
          expect_mode="price")

    # Precios baratos: muchos modos price y base_rate.
    for price in CHEAP_PRICES:
        for n in (1, 5, 20, 40, 60, 120, 300):
            b.add("cheap-lines", f"{n}x{price}", make_input([line("1", price) for _ in range(n)]))
    for i in range(20):
        n = rng.randint(30, 400)
        lines = [line(str(rng.randint(1, 3)), rng.choice(CHEAP_PRICES)) for _ in range(n)]
        b.add("cheap-lines", f"random-{i:02d}-{n}lines", make_input(lines, rand_global(rng, make_input(lines))))

    # Descuento global de 0.01 en carritos de muchas líneas baratas.
    for price in CHEAP_PRICES + ("0.01", "0.05", "0.99"):
        for n in (10, 77, 250, 1000):
            b.add("global-0.01", f"{n}x{price}", make_input([line("1", price) for _ in range(n)], "0.01"))
    for i in range(8):
        n = rng.randint(20, 300)
        lines = [line("1", rng.choice(CHEAP_PRICES),
                      afectacion=rng.choice(("gravado", "gravado", "exonerado", "inafecto")))
                 for _ in range(n)]
        b.add("global-0.01", f"mixed-{i}-{n}lines", make_input(lines, "0.01"))

    # Empate de tres grupos iguales: los céntimos sueltos van a gravado primero.
    for price in ("10.00", "1.18", "0.10", "0.43"):
        three = [line("1", price, afectacion=a) for a in ("gravado", "exonerado", "inafecto")]
        for gd in ("0.01", "0.02", "0.03", "0.04", "0.05"):
            cap = cap_of(make_input(three))
            if Decimal(gd) <= cap:
                b.add("three-way-tie", f"{price}-global-{gd}", make_input(three, gd))
    # Empate con otro orden de líneas (el orden de grupos es fijo, no el de líneas).
    rev = [line("1", "10.00", afectacion=a) for a in ("inafecto", "exonerado", "gravado")]
    b.add("three-way-tie", "reversed-lines-global-0.02", make_input(rev, "0.02"))
    # Empate solo entre exonerado e inafecto.
    two = [line("1", "5.00", afectacion=a) for a in ("exonerado", "inafecto")]
    b.add("three-way-tie", "exo-ina-only-global-0.01", make_input(two, "0.01"))

    # Descuento total: global igual al tope y líneas descontadas por completo.
    full_carts = {
        "single-gravado": [line("1", "118.00")],
        "mixed": [line("2", "10.00"), line("1", "5.55", afectacion="exonerado"),
                  line("3", "0.43", afectacion="inafecto")],
        "with-free": [line("1", "7.77"), line("1", "50.00", afectacion="gratuito")],
        "base-rate": [line("1", "0.43") for _ in range(100)],
        "cheap-mixed": [line("1", "0.16") for _ in range(70)] + [line("1", "0.16", afectacion="exonerado")],
    }
    for name, lines in full_carts.items():
        cap = cap_of(make_input(lines))
        b.add("full-discount", f"global-{name}", make_input(lines, fmt(cap, 2)))
    b.add("full-discount", "line-gravado", make_input([line("2", "12.50", "25.00")]))
    b.add("full-discount", "line-gravado-plus-paid",
          make_input([line("3", "4.20", "12.60"), line("1", "9.90")]))
    b.add("full-discount", "line-exonerado", make_input([line("1", "8.00", "8.00", "exonerado")]))
    # En base_rate una cantidad enorme deja un residuo de céntimos por el VU de
    # 10 decimales: RB = 0.03 y RI = 0.01 aunque PT sea 0 (§3).
    kind, res = b.add("full-discount", "line-gravado-base-rate-huge-qty-residue",
                      make_input([line("1000000000", "0.01", "10000000.00")]
                                 + [line("1", "0.43") for _ in range(100)]),
                      expect_mode="base_rate")
    assert res["lines"][0]["base"] == Decimal("0.03") and res["lines"][0]["igv"] == Decimal("0.01")
    kind, res = b.add("full-discount", "line-gravado-price-huge-qty",
                      make_input([line("1000000000", "0.01", "10000000.00")]), expect_mode="price")
    assert res["lines"][0]["total"] == 0 and res["lines"][0]["base"] == 0 and res["lines"][0]["igv"] == 0
    b.add("full-discount", "line-gravado-base-rate",
          make_input([line("1", "1.00", "1.00")] + [line("1", "0.43") for _ in range(100)]),
          expect_mode="base_rate")

    # Una línea gratuita en cada modo.
    b.add("free", "single-free-price", make_input([line("1", "59.00", afectacion="gratuito")]),
          expect_mode="price")
    b.add("free", "free-plus-gravado-price",
          make_input([line("2", "25.00"), line("1", "11.80", afectacion="gratuito")], "1.00"),
          expect_mode="price")
    b.add("free", "free-plus-gravado-base-rate",
          make_input([line("1", "0.43") for _ in range(80)] + [line("1", "11.80", afectacion="gratuito")],
                     "0.05"),
          expect_mode="base_rate")
    b.add("free", "free-with-discount-base-rate",
          make_input([line("1", "0.10") for _ in range(60)]
                     + [line("3", "0.43", "0.10", afectacion="gratuito")]),
          expect_mode="base_rate")
    b.add("free", "free-with-discount-price",
          make_input([line("1", "3.00"), line("4", "2.35", "1.11", afectacion="gratuito")]),
          expect_mode="price")
    b.add("free", "free-4dp-quantity",
          make_input([line("1.2345", "9.9999", afectacion="gratuito"), line("1", "1.00")]))

    # USD: el cálculo es idéntico al de PEN.
    usd_carts = [
        ([line("1", "100.00")], "3.750"),
        ([line("3", "19.99", "2.00"), line("1", "5.00", afectacion="exonerado")], "3.8"),
        ([line("1", "0.43") for _ in range(90)], "3.712"),
        ([line("2.5", "4.1234"), line("1", "7.00", afectacion="inafecto")], "999.999"),
        ([line("1", "1.00")], "0.001"),
        ([line("1", "10.00"), line("1", "1.00", afectacion="gratuito")], "4"),
    ]
    for i, (lines, rate) in enumerate(usd_carts):
        gd = rand_global(rng, make_input(lines))
        b.add("usd", f"cart-{i}-rate-{rate}", make_input(lines, gd, "0", "USD", rate))
        b.add("usd", f"cart-{i}-pen-twin", make_input(lines, gd))
    for i in range(6):
        lines = [rand_line(rng) for _ in range(rng.randint(1, 8))]
        rate = dec_str(rng.randint(3000, 4200), 3)
        b.add("usd", f"random-{i}", make_input(lines, rand_global(rng, make_input(lines)),
                                                dec_str(rng.randint(0, 500), 2), "USD", rate))

    # Cantidades con 4 decimales.
    for i in range(16):
        lines = []
        for _ in range(rng.randint(1, 6)):
            qty = dec_str(rng.randint(1, 999999), 4)
            price = rng.choice((rand_price(rng), "0.43", "12.3456", "0.0001"))
            lines.append(line(qty, price, "0", rng.choice(("gravado", "gravado", "exonerado", "inafecto"))))
        b.add("qty-4dp", f"random-{i:02d}", make_input(lines, rand_global(rng, make_input(lines))))
    b.add("qty-4dp", "0.0001x0.0001", make_input([line("0.0001", "0.0001")]))
    b.add("qty-4dp", "0.0001x9999999999.9999", make_input([line("0.0001", "9999999999.9999")]))
    b.add("qty-4dp", "9999999999.9999x0.0001", make_input([line("9999999999.9999", "0.0001")]))
    b.add("qty-4dp", "0.3333x3.00", make_input([line("0.3333", "3.00")]))

    # Ceros.
    b.add("zeros", "price-zero-gravado", make_input([line("1", "0")]))
    b.add("zeros", "price-zero-all-affectations",
          make_input([line("1", "0", afectacion=a) for a in AFECTACIONES]))
    b.add("zeros", "price-zero-with-other-charges", make_input([line("5", "0.00")], "0", "3.50"))
    b.add("zeros", "zero-plus-paid-global", make_input([line("1", "0"), line("1", "2.00")], "2.00"))
    b.add("zeros", "only-free-zero-global", make_input([line("1", "10.00", afectacion="gratuito")], "0"))
    b.add("zeros", "explicit-zero-strings",
          make_input([line("1", "1.00", "0.00")], "0.00", "0.00"))
    b.add("zeros", "group-zero-gets-no-cent",
          make_input([line("1", "1.00"), line("1", "0", afectacion="exonerado"),
                      line("1", "1.00", afectacion="inafecto")], "0.01"))

    # Otros cargos.
    for oc in ("0.01", "5.00", "123.45"):
        b.add("other-charges", f"oc-{oc}", make_input([line("2", "3.40"), line("1", "1.00", afectacion="exonerado")],
                                                      "0.50", oc))

    # Mezcla de afectaciones con descuento global que deja céntimos sueltos.
    for i in range(30):
        lines = [rand_line(rng, aff=a) for a in ("gravado", "exonerado", "inafecto")]
        lines += [rand_line(rng) for _ in range(rng.randint(0, 5))]
        inp = make_input(lines)
        cap_c = cents(cap_of(inp))
        # Un descuento primo con los totales casi nunca divide exacto.
        gd = dec_str(rng.randint(1, max(1, min(cap_c, 997))), 2) if cap_c else "0"
        b.add("mixed-leftover", f"random-{i:02d}", make_input(lines, gd, rng.choice(("0", "1.00"))))
    b.add("mixed-leftover", "1-1-1-global-0.02",
          make_input([line("1", "1.00", afectacion=a) for a in ("gravado", "exonerado", "inafecto")], "0.02"))
    b.add("mixed-leftover", "3-2-2-global-0.05",
          make_input([line("1", "3.00"), line("1", "2.00", afectacion="exonerado"),
                      line("1", "2.00", afectacion="inafecto")], "0.05"))
    b.add("mixed-leftover", "7-11-13-global-0.10",
          make_input([line("1", "0.07"), line("1", "0.11", afectacion="exonerado"),
                      line("1", "0.13", afectacion="inafecto")], "0.10"))

    # Carritos aleatorios generales.
    for i in range(70):
        lines = [rand_line(rng) for _ in range(rng.randint(1, 15))]
        inp = make_input(lines)
        kind, _ = compute(inp, check_invariants=False)
        gd = rand_global(rng, inp) if kind == "want" else "0"
        oc = rng.choice(("0", "0", "0", dec_str(rng.randint(1, 10000), 2)))
        b.add("random", f"cart-{i:03d}", make_input(lines, gd, oc))

    # Escala del valor: ceros finales de más se aceptan (§1).
    b.add("trailing-zeros", "quantity-1.00000", make_input([line("1.00000", "1.00")]))
    b.add("trailing-zeros", "quantity-2.500000000", make_input([line("2.500000000", "4.10")]))
    b.add("trailing-zeros", "unitprice-1.000000", make_input([line("1", "1.000000")]))
    b.add("trailing-zeros", "unitprice-0.43000000", make_input([line("3", "0.43000000")]))
    b.add("trailing-zeros", "discount-0.000", make_input([line("1", "1.00", "0.000")]))
    b.add("trailing-zeros", "discount-0.5000", make_input([line("1", "1.00", "0.5000")]))
    b.add("trailing-zeros", "exonerado-unitprice-2.50000",
          make_input([line("1", "2.50000", afectacion="exonerado")]))
    b.add("trailing-zeros", "global-0.000", make_input([line("1", "1.00")], "0.000"))
    b.add("trailing-zeros", "global-0.0100", make_input([line("1", "1.00")], "0.0100"))
    b.add("trailing-zeros", "other-charges-5.0000", make_input([line("1", "1.00")], "0", "5.0000"))
    b.add("trailing-zeros", "rate-3.7500", make_input([line("1", "1.00")], currency="USD", exchange_rate="3.7500"))
    b.add("trailing-zeros", "rate-999.999000", make_input([line("1", "1.00")], currency="USD",
                                                         exchange_rate="999.999000"))
    # Decimales de más significativos: se rechazan aunque haya ceros al final.
    E = "validation_failed"
    b.add("trailing-zeros", "quantity-1.000010-rejected", make_input([line("1.000010", "1.00")]),
          (E, "lines[0].quantity"))
    b.add("trailing-zeros", "unitprice-1.000010-rejected", make_input([line("1", "1.000010")]),
          (E, "lines[0].unitPrice"))
    b.add("trailing-zeros", "discount-0.0010-rejected", make_input([line("1", "1.00", "0.0010")]),
          (E, "lines[0].discount"))
    b.add("trailing-zeros", "global-0.0010-rejected", make_input([line("1", "1.00")], "0.0010"),
          (E, "globalDiscount"))
    b.add("trailing-zeros", "other-charges-1.0050-rejected", make_input([line("1", "1.00")], "0", "1.0050"),
          (E, "otherCharges"))
    b.add("trailing-zeros", "rate-3.75010-rejected", make_input([line("1", "1.00")], currency="USD",
                                                               exchange_rate="3.75010"), (E, "exchangeRate"))

    # --- errores del §2 ---------------------------------------------------------
    ok = [line("1", "10.00")]
    E = "validation_failed"
    b.add("error-currency", "eur", make_input(ok, currency="EUR"), (E, "currency"))
    b.add("error-currency", "lowercase-pen", make_input(ok, currency="pen"), (E, "currency"))
    b.add("error-currency", "eur-with-rate-first-wins", make_input(ok, currency="EUR", exchange_rate="3.750"),
          (E, "currency"))
    b.add("error-currency", "pen-with-rate", make_input(ok, exchange_rate="3.750"), (E, "exchangeRate"))
    b.add("error-currency", "usd-without-rate", make_input(ok, currency="USD"),
          ("exchange_rate_required", "exchangeRate"))
    for name, rate in (("zero", "0"), ("negative", "-3.750"), ("over-max", "1000.000"),
                       ("over-max-by-0.001", "1000"), ("4-decimals", "3.7505")):
        b.add("error-currency", f"usd-rate-{name}", make_input(ok, currency="USD", exchange_rate=rate),
              (E, "exchangeRate"))
    # El orden manda: el tipo de cambio falla antes que una línea inválida.
    b.add("error-currency", "usd-without-rate-and-bad-line",
          make_input([line("0", "1.00")], currency="USD"), ("exchange_rate_required", "exchangeRate"))

    b.add("error-lines", "no-lines", make_input([]), (E, "lines"))
    b.add("error-lines", "1001-lines", make_input([line("1", "1.00") for _ in range(1001)]), (E, "lines"))
    b.add("error-lines", "1000-lines-ok", make_input([line("1", "0.16") for _ in range(1000)]))

    bad_line_cases = [
        ("quantity-zero", line("0", "1.00"), "quantity"),
        ("quantity-negative", line("-1", "1.00"), "quantity"),
        ("quantity-over-max", line("10000000000", "0.0001"), "quantity"),
        ("quantity-5-decimals", line("1.00001", "1.00"), "quantity"),
        ("unitprice-negative", line("1", "-0.01"), "unitPrice"),
        ("unitprice-over-max", line("1", "10000000000"), "unitPrice"),
        ("unitprice-5-decimals", line("1", "1.00001"), "unitPrice"),
        ("discount-negative", line("1", "1.00", "-0.01"), "discount"),
        ("discount-3-decimals", line("1", "1.00", "0.001"), "discount"),
        ("gross-over-max", line("1000", "1000000000"), "unitPrice"),
        ("gross-over-max-by-a-hair", line("100.0001", "9999999999.9999"), "unitPrice"),
        ("discount-over-gross", line("2", "1.00", "2.01"), "discount"),
        ("discount-over-unrounded-gross", line("0.3333", "1.00", "0.34"), "discount"),
        ("afectacion-invalid", line("1", "1.00", afectacion="exportacion"), "afectacion"),
        ("afectacion-uppercase", line("1", "1.00", afectacion="GRAVADO"), "afectacion"),
        ("quantity-before-unitprice", line("0", "-1"), "quantity"),
        ("unitprice-before-discount", line("1", "-1", "-1"), "unitPrice"),
        ("discount-before-afectacion", line("1", "1.00", "5.00", "otro"), "discount"),
    ]
    for name, bad, field in bad_line_cases:
        b.add("error-line", f"{name}-at-0", make_input([bad]), (E, f"lines[0].{field}"))
    for name, bad, field in bad_line_cases[:6]:
        b.add("error-line", f"{name}-at-2", make_input([line("1", "1.00"), line("2", "3.00"), bad]),
              (E, f"lines[2].{field}"))
    b.add("error-line", "first-bad-line-wins",
          make_input([line("1", "1.00"), line("1", "1.00", afectacion="x"), line("0", "1.00")]),
          (E, "lines[1].afectacion"))
    # Bordes válidos que no deben fallar.
    b.add("error-line", "discount-equal-unrounded-gross-ok", make_input([line("0.5", "0.02", "0.01")]))
    b.add("error-line", "gross-equal-max-ok",
          make_input([line("100", "9999999999.9999", "0", "exonerado")]))

    for field in ("globalDiscount", "otherCharges"):
        for name, val in (("negative", "-0.01"), ("3-decimals", "0.001"), ("over-max", "1000000000000.00")):
            gd = val if field == "globalDiscount" else "0"
            oc = val if field == "otherCharges" else "0"
            b.add("error-charges", f"{field}-{name}", make_input(ok, gd, oc), (E, field))
    b.add("error-charges", "globalDiscount-before-otherCharges", make_input(ok, "-1", "-1"), (E, "globalDiscount"))
    b.add("error-charges", "bad-line-before-globalDiscount", make_input([line("0", "1")], "-1"),
          (E, "lines[0].quantity"))

    # §2.d: tope del descuento global (los gratuitos no cuentan).
    b.add("error-after", "global-over-cap", make_input([line("1", "10.00")], "10.01"), (E, "globalDiscount"))
    b.add("error-after", "global-over-cap-free-not-counted",
          make_input([line("1", "1.00"), line("1", "100.00", afectacion="gratuito")], "1.01"),
          (E, "globalDiscount"))
    b.add("error-after", "global-on-free-only",
          make_input([line("1", "100.00", afectacion="gratuito")], "0.01"), (E, "globalDiscount"))
    b.add("error-after", "global-over-fully-discounted-line",
          make_input([line("1", "5.00", "5.00")], "0.01"), (E, "globalDiscount"))
    b.add("error-after", "global-over-cap-base-rate",
          make_input([line("1", "0.43") for _ in range(100)], "43.01"), (E, "globalDiscount"))

    # §2.d: desborde de los totales, en el orden del enunciado.
    # 100 × 9999999999.9999 = 999999999999.99: el bruto máximo de una línea.
    def big(aff="gravado"):
        return line("100", "9999999999.9999", "0", aff)

    b.add("error-after", "totals-gravado",
          make_input([big(), big()]), (E, "totals.gravado"))
    b.add("error-after", "totals-exonerado",
          make_input([big("exonerado"), line("1", "0.01", afectacion="exonerado")]),
          (E, "totals.exonerado"))
    b.add("error-after", "totals-inafecto",
          make_input([big("inafecto"), line("1", "0.01", afectacion="inafecto")]),
          (E, "totals.inafecto"))
    b.add("error-after", "totals-gratuito",
          make_input([big("gratuito"), big("gratuito"), line("1", "1.00")]),
          (E, "totals.gratuito"))
    b.add("error-after", "totals-subtotal",
          make_input([big("exonerado"), line("1", "0.01", afectacion="inafecto")]),
          (E, "totals.subtotal"))
    b.add("error-after", "totals-total-by-other-charges",
          make_input([big()], "0", "0.01"), (E, "totals.total"))
    b.add("error-after", "totals-total-by-igv",
          make_input([big(), line("100", "1000000000", afectacion="exonerado")]),
          (E, "totals.total"))
    b.add("error-after", "global-cap-before-totals",
          make_input([big("gratuito"), big("gratuito"), line("1", "1.00")], "1.01"),
          (E, "globalDiscount"))
    b.add("error-after", "max-total-ok", make_input([big()]))

    return b


def main():
    if len(sys.argv) != 2:
        sys.exit("usage: python3 -I gen_vectors.py vectors.json")
    with localcontext(CTX):
        b = build()
    assert len(b.vectors) >= 250, len(b.vectors)
    with open(sys.argv[1], "w", encoding="utf-8", newline="\n") as f:
        json.dump(b.vectors, f, indent=2, ensure_ascii=False)
        f.write("\n")
    for cat in sorted(b.categories):
        print(f"{cat}: {b.categories[cat]}", file=sys.stderr)
    print(f"total: {len(b.vectors)}", file=sys.stderr)


if __name__ == "__main__":
    main()
