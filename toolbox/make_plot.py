import os
import json
import matplotlib.pyplot as plt

BASE_DIR = "/Users/dmitry/Desktop/hl_homework_"

WHEN = "before"

with open(os.path.join(BASE_DIR, f"user_search_report_{WHEN}_index.json")) as f:
    data = json.load(f)

concs = [1, 10, 100, 1000]
lat_means = []
throughputs = []

for c in concs:
    latencies = data[f'{WHEN}_index_conc_{c}']['latencies_ms']
    lat_means.append(sum(latencies)/len(latencies))
    throughputs.append(data[f'{WHEN}_index_conc_{c}']['throughput'])

plt.figure()
plt.plot(concs, lat_means, marker='o')
plt.xlabel('Concurrency')
plt.ylabel('Mean Latency (ms)')
plt.title(f'Latency {WHEN} index')
plt.xscale('log')
plt.savefig(os.path.join(BASE_DIR, f'latency_{WHEN}_index.png'))

plt.figure()
plt.plot(concs, throughputs, marker='o')
plt.xlabel('Concurrency')
plt.ylabel('Throughput (req/sec)')
plt.title(f'Throughput {WHEN} index')
plt.xscale('log')
plt.savefig(os.path.join(BASE_DIR, f'throughput_{WHEN}_index.png'))