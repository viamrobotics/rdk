<script lang="ts">
import { Line } from "vue-chartjs";
import { Component, Prop, Vue } from "vue-property-decorator";
import "chartjs-plugin-streaming";
import { Chart } from "chart.js";
import { MotorServicePIDStepResponse } from "proto/api/component/v1/motor_pb";

@Component({
  extends: Line,
})
export default class PIDChart extends Vue {
  @Prop() labels?: [string];
  @Prop() colors?: [string];

  private options: Chart.ChartOptions = {};
  public datasets: Chart.ChartData = {};
  private chart!: Chart;

  public renderChart!: (
    chartData: Chart.ChartData,
    options: Chart.ChartOptions
  ) => void;

  mounted(): void {
    this.buildDataSets();
    this.renderChart(this.datasets, this.pidOptions());
  }
  private pidOptions() {
    var opt = {
      animation: false, // disable animations
      plugins: {
        streaming: {
          frameRate: 20, // chart is drawn 10 times every second
        },
      },
      scales: {
        xAxes: [
          {
            type: "realtime",
            realtime: {
              delay: 0,
              duration: 7000,
              ttl: undefined,
              frameRate: 20,
              pause: true,
            },
          },
        ],
      },
    } as unknown as Chart.ChartOptions;
    return opt;
  }
  private buildDataSets(): void {
    this.datasets.labels = this.labels;
    this.datasets.datasets = [];
    this.colors?.forEach(
      function (this: PIDChart, value: string, idx: number) {
        (this.datasets?.datasets as Chart.ChartDataSets[]).push({
          data: [],
          backgroundColor: value,
          borderColor: value,
          label: this.labels?.[idx],
          fill: false,
          pointRadius: 0,
          showLine: true,
        });
      }.bind(this)
    );
  }
  public addData(data: MotorServicePIDStepResponse): void {
    this.$data?.datasets?.datasets?.forEach((dataset: Chart.ChartDataSets) => {
      if (dataset.label == "Set Point") {
        (dataset.data as Chart.ChartPoint[]).push({
          x: Date.now(),
          y: data.getSetPoint(),
        });
      } else if (dataset.label == "Reference Value") {
        (dataset.data as Chart.ChartPoint[]).push({
          x: Date.now(),
          y: data.getRefValue(),
        });
      }
    });
  }
  public pause(): void {
    if (this.$data._chart.options.scales.xAxes[0].realtime.pause) {
      this.$data._chart.options.scales.xAxes[0].realtime.pause = true;
    }
  }
  public unpause(): void {
    if (this.$data._chart.options.scales.xAxes[0].realtime.pause) {
      this.$data._chart.options.scales.xAxes[0].realtime.pause = false;
    }
  }
  public update(s: string): void {
    this.$data._chart.update(s);
  }
}
</script>
