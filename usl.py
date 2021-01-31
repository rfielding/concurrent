#!/usr/bin/python3
import sys
import math
import numpy as np
from pyusl import usl
import matplotlib.pyplot as plt

alpha=float(sys.argv[1])
beta=float(sys.argv[2])
gamma=float(sys.argv[3])

u,x,y = usl(), [], []
with open('data.txt','r') as reader:
    reader.readline()
    for line in reader.readlines():
        line = line.strip()
        x.append(float(line.split(',')[0]))
        y.append(float(line.split(',')[1]))
u.fit(x,y,requires_plot=False)
xgrid = np.linspace(1,2*len(x),2*len(x))
ygrid = u.compute(xgrid)

plt.plot(xgrid, ygrid, 'r-', color='red',
        label='USL(py): gamma=%5.6f, alpha=%5.6f, beta=%5.6f'
          % (u.gamma, u.alpha, u.beta))

y2grid = (xgrid*gamma)/(1 + alpha*(xgrid-1) + beta*xgrid*(xgrid-1))
plt.plot(xgrid, y2grid, 'r-', color='green',
        label='USL(go - weighted): gamma=%5.6f, alpha=%5.6f, beta=%5.6f'
          % (gamma, alpha, beta))

if beta > 0:
  gp=math.sqrt((1-alpha)/beta)
  if gp < len(xgrid):
    plt.axvline(gp,color='green')
if u.beta > 0:
  rp=math.sqrt((1-u.alpha)/u.beta)
  if rp < len(xgrid):
    plt.axvline(rp,color='red')

plt.xlabel('load')
plt.ylabel('throughput',color='red')
plt.plot(x,y, 'r*', color='purple',label="measured")


plt.legend()
plt.show()
