import argparse
import os
import json
import matplotlib.pyplot as plt


def make_plot(input_file, output_dir):
    with open(input_file) as f:
        data = json.load(f)

    concs = [1, 10, 100, 1000]
    lat_means = []
    throughputs = []

    for c in concs:
        latencies = data[f"conc_{c}"]["latencies_ms"]
        lat_means.append(sum(latencies) / len(latencies))
        throughputs.append(data[f"conc_{c}"]["throughput"])

    plt.figure()
    plt.plot(concs, lat_means, marker="o")
    plt.xlabel("Concurrency")
    plt.ylabel("Mean Latency (ms)")
    plt.title(f"Latency index")
    plt.xscale("log")
    plt.savefig(os.path.join(output_dir, f"latency_index.png"))

    plt.figure()
    plt.plot(concs, throughputs, marker="o")
    plt.xlabel("Concurrency")
    plt.ylabel("Throughput (req/sec)")
    plt.title(f"Throughput index")
    plt.xscale("log")
    plt.savefig(os.path.join(output_dir, f"throughput_index.png"))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Генерация графиков по отчету поиска пользователей")
    parser.add_argument("--input", type=str, required=True, help="Путь к отчету")
    parser.add_argument("--output", type=str, required=True, default="./reports", help="Путь к каталогу с результатами")

    args = parser.parse_args()
    if not os.path.exists(args.output):
        os.makedirs(args.output)
    make_plot(input_file=args.input, output_dir=args.output)
    print(f"Plots saved in {args.output}")
