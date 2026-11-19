# Oráculo independiente (Python Decimal/Fraction, sin código compartido con el paquete Go)
# que genera testdata/vectors.json: python3 -I gen_vectors.py vectors.json
import json, random, sys
from decimal import Decimal as D, getcontext, ROUND_HALF_UP
from fractions import Fraction
getcontext().prec = 80
def q(x, n): return x.quantize(D(1).scaleb(-n), rounding=ROUND_HALF_UP)
def f2(x): return format(q(x, 2), 'f')
DIV = D('1.18'); RATE = D('0.18')

def compute(inp):
    cur = inp['currency']
    lines = inp['lines']
    mode_dev = D(0)
    cs = []
    for l in lines:
        qty, price, disc = D(l['quantity']), D(l['unitPrice']), D(l['discount'])
        pt = q(qty*price - disc, 2)
        unit = q(price/DIV, 10)
        c = dict(pt=pt, unit=unit)
        if l['afectacion'] == 'gravado':
            rb = q(qty*unit - q(disc/DIV, 10), 2)
            if rb < 0: rb = D(0)
            ri = q(rb*RATE, 2)
            c['rb'], c['ri'] = rb, ri
            mode_dev += abs(pt - (rb+ri))
        cs.append(c)
    mode = 'base_rate' if mode_dev > D('0.50') else 'price'
    out_lines = []; gb = gg = eg = ig = fb = fi = D(0); gi_rate = D(0)
    for l, c in zip(lines, cs):
        a = l['afectacion']
        if a == 'gravado':
            if mode == 'price':
                t = c['pt']; b = q(t/DIV, 2); i = t-b
            else:
                b, i = c['rb'], c['ri']; t = b+i
            gb += b; gg += t; gi_rate += i
        elif a == 'exonerado':
            b = t = c['pt']; i = D(0); eg += t
        elif a == 'inafecto':
            b = t = c['pt']; i = D(0); ig += t
        else:
            b = q(c['pt']/DIV, 2); i = c['pt']-b; t = D(0); fb += b; fi += i
        out_lines.append(dict(unitValue=format(c['unit'], 'f'), base=f2(b), igv=f2(i), total=f2(t)))
    gd = D(inp['globalDiscount']); S = gg+eg+ig
    if gd > S: return None
    groups = [gg, eg, ig]; shares = [D(0)]*3
    if gd > 0 and S > 0:
        dc = int(gd*100); gc = [int(g*100) for g in groups]; sc = sum(gc)
        fr = [Fraction(dc*g, sc) for g in gc]
        base_c = [int(x.numerator//x.denominator) for x in fr]
        left = dc - sum(base_c)
        order = sorted(range(3), key=lambda k: (-(fr[k]-base_c[k]), k))
        for k in order:
            if left > 0 and gc[k] > 0: base_c[k] += 1; left -= 1
        shares = [D(c)/100 for c in base_c]
    ga, gi = gb, gg-gb
    if mode == 'base_rate': gi = gi_rate
    if shares[0] != 0:
        if shares[0] == gg: ga = gi = D(0)
        elif mode == 'price':
            net = gg-shares[0]; ga = q(net/DIV, 2); gi = net-ga
        else:
            ga = gb - q(shares[0]/DIV, 2)
            if ga < 0: ga = D(0)
            gi = q(ga*RATE, 2)
    disc = (gb-ga)+shares[1]+shares[2]
    sub = gb+eg+ig; oc = D(inp['otherCharges'])
    tot = sub-disc+gi+oc
    return dict(mode=mode, lines=out_lines, totals=dict(gravado=f2(gb), exonerado=f2(eg), inafecto=f2(ig),
        gratuito=f2(fb), subtotal=f2(sub), discounts=f2(disc), igv=f2(gi), freeIgv=f2(fi), otherCharges=f2(oc), total=f2(tot)))

vecs = []
def add(name, inp, err=None):
    v = dict(name=name, input=inp)
    if err: v['error'] = err
    else:
        w = compute(inp); assert w is not None, name; v['want'] = w
    vecs.append(v)
def L(qty, price, aff='gravado', disc='0'): return dict(quantity=qty, unitPrice=price, discount=disc, afectacion=aff)
def I(lines, cur='PEN', gd='0', oc='0', rate=None):
    i = dict(currency=cur, lines=lines, globalDiscount=gd, otherCharges=oc)
    if rate: i['exchangeRate'] = rate
    return i

# Redondeo y escala (carritos de una línea)
for p in ['118', '100', '10', '9.99', '0.01', '0.0001', '1.18', '5.90', '59', '0.5', '1234.5678', '33.33', '2.95', '0.10', '1', '3.5', '7.77']:
    add(f'gravado single price {p}', I([L('1', p)]))
for qty in ['0.0001', '0.5', '1.5', '2.25', '3', '10', '12.3456', '100', '999.9999', '0.3333']:
    add(f'gravado qty {qty} price 11.8', I([L(qty, '11.8')]))
    add(f'gravado qty {qty} price 3.35', I([L(qty, '3.35')]))
# Afectaciones
for aff in ['gravado', 'exonerado', 'inafecto', 'gratuito']:
    for qty, p in [('1', '118'), ('3', '2.5'), ('0.75', '40'), ('7', '0.35')]:
        add(f'{aff} qty {qty} price {p}', I([L(qty, p, aff)]))
    add(f'{aff} with line discount', I([L('2', '50', aff, '10.50')]))
    add(f'{aff} fully discounted line', I([L('2', '50', aff, '100')]))
# Mezcla
add('mixed all four', I([L('1', '118'), L('2', '50', 'exonerado'), L('3', '10', 'inafecto'), L('4', '25', 'gratuito')]))
add('mixed with global discount', I([L('1', '118'), L('2', '50', 'exonerado'), L('3', '10', 'inafecto')], gd='15.55'))
add('global discount only free lines zero', I([L('1', '100', 'gratuito')]))
add('other charges', I([L('1', '118')], oc='2.50'))
add('other charges and discount', I([L('2', '59'), L('1', '10', 'exonerado')], gd='3', oc='0.30'))
# Descuentos globales
for gd in ['0.01', '0.50', '1', '10', '59', '117.99', '118']:
    add(f'global discount {gd} on 118', I([L('1', '118')], gd=gd))
    add(f'global discount {gd} on two groups', I([L('1', '100'), L('1', '18', 'exonerado')], gd=gd if float(gd) <= 118 else '1'))
add('global discount tie breaker', I([L('1', '10'), L('1', '10', 'exonerado'), L('1', '10', 'inafecto')], gd='0.01'))
add('global discount tie breaker 2 cents', I([L('1', '10'), L('1', '10', 'exonerado'), L('1', '10', 'inafecto')], gd='0.02'))
add('global discount full', I([L('1', '100'), L('1', '50', 'exonerado')], gd='150'))
# USD
for r in ['3.756', '3.7', '1', '0.001', '999.999']:
    add(f'USD rate {r}', I([L('3', '12.34'), L('1', '5', 'exonerado')], cur='USD', rate=r))
# Cantidades decimales / precios con 4 decimales
for k in range(12):
    add(f'decimal quantities {k}', I([L(f'{k+1}.{(k*37+11)%10000:04d}', f'{(k*13+7)%90+1}.{(k*911+3)%10000:04d}')]))
# Desvío que cruza 0.50: muchas líneas de 1 unidad a precio con redondeo adverso
for n in (20, 40, 60, 100):
    for p in ('0.99', '0.10', '0.16', '0.23', '0.29', '0.03'):
        add(f'many lines {n} price {p}', I([L('1', p) for _ in range(n)]))
for n in (49, 50, 51, 52):
    add(f'deviation boundary {n} lines of 0.10', I([L('1', '0.10') for _ in range(n)]))
add('deviation 0.50 plus exonerado and discount', I([L('1', '0.10') for _ in range(51)] + [L('2', '5', 'exonerado')], gd='1.25'))
add('deviation crosses with USD', I([L('1', '0.16') for _ in range(70)], cur='USD', rate='3.756'))
add('deviation crosses with free lines ignored', I([L('1', '0.10') for _ in range(40)] + [L('1', '0.10', 'gratuito') for _ in range(40)]))
add('many lines 200 mix', I([L('1', '1.33', 'gravado' if k % 5 else 'exonerado') for k in range(200)]))

# Aleatorios con semilla fija
rnd = random.Random(167)
affs = ['gravado']*6 + ['exonerado', 'inafecto', 'gratuito']
for k in range(130):
    n = rnd.choice([1, 2, 3, 5, 8, 15, 30, 60, 90])
    lines = []
    for _ in range(n):
        qty = f'{rnd.randint(1, 2000)/rnd.choice([1,10,100,10000]):.4f}'.rstrip('0').rstrip('.') or '1'
        if D(qty) <= 0: qty = '1'
        price = f'{rnd.randint(1, 5000000)/rnd.choice([100,1000,10000]) if rnd.random() < .8 else rnd.randint(1,40)/100:.4f}'.rstrip('0').rstrip('.')
        a = rnd.choice(affs)
        gross = D(qty)*D(price)
        disc = '0'
        if rnd.random() < 0.3: disc = f2(gross*D(rnd.randint(0, 100))/100) if gross < D('1e9') else '0'
        if D(disc) > gross: disc = '0'
        lines.append(L(qty, price, a, disc))
    inp = I(lines, cur=rnd.choice(['PEN', 'PEN', 'USD']), oc=rnd.choice(['0', '0', '1.50', '10']))
    if inp['currency'] == 'USD': inp['exchangeRate'] = rnd.choice(['3.756', '3.812', '3.7'])
    S = sum(q(D(l['quantity'])*D(l['unitPrice']) - D(l['discount']), 2) for l in lines if l['afectacion'] != 'gratuito')
    if rnd.random() < 0.4 and S > 0:
        inp['globalDiscount'] = f2(S*D(rnd.randint(0, 100))/100)
    add(f'random {k}', inp)

# Errores
add('err usd without rate', I([L('1', '10')], cur='USD'), dict(code='exchange_rate_required', field='exchangeRate'))
add('err pen with rate', I([L('1', '10')], rate='3.756'), dict(code='validation_failed', field='exchangeRate'))
add('err rate 4 decimals', I([L('1', '10')], cur='USD', rate='3.7561'), dict(code='validation_failed', field='exchangeRate'))
add('err rate zero', I([L('1', '10')], cur='USD', rate='0'), dict(code='validation_failed', field='exchangeRate'))
add('err currency', I([L('1', '10')], cur='EUR'), dict(code='validation_failed', field='currency'))
add('err no lines', I([]), dict(code='validation_failed', field='lines'))
add('err zero quantity', I([L('0', '10')]), dict(code='validation_failed', field='lines[0].quantity'))
add('err negative quantity', I([L('-1', '10')]), dict(code='validation_failed', field='lines[0].quantity'))
add('err negative price', I([L('1', '-10')]), dict(code='validation_failed', field='lines[0].unitPrice'))
add('err quantity 5 decimals', I([L('1.00001', '10')]), dict(code='validation_failed', field='lines[0].quantity'))
add('err discount exceeds', I([L('1', '10', disc='10.01')]), dict(code='validation_failed', field='lines[0].discount'))
add('err afectacion', I([L('1', '10', 'otro')]), dict(code='validation_failed', field='lines[0].afectacion'))
add('err global discount exceeds', I([L('1', '10')], gd='10.01'), dict(code='validation_failed', field='globalDiscount'))
add('err global discount on free only', I([L('1', '10', 'gratuito')], gd='1'), dict(code='validation_failed', field='globalDiscount'))
add('err negative other charges', I([L('1', '10')], oc='-1'), dict(code='validation_failed', field='otherCharges'))
add('err line over max', I([L('9999999999', '9999999999')]), dict(code='validation_failed', field='lines[0].unitPrice'))

with open(sys.argv[1], 'w') as fh:
    fh.write('[\n' + ',\n'.join(json.dumps(v, separators=(',', ':'), ensure_ascii=False) for v in vecs) + '\n]\n')
modes = [v['want']['mode'] for v in vecs if 'want' in v]
print(len(vecs), 'vectors;', modes.count('base_rate'), 'base_rate;', sum(1 for v in vecs if 'error' in v), 'errors')
