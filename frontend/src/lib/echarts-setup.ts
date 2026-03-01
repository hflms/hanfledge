/**
 * Unified ECharts setup — import this instead of echarts/core directly.
 *
 * Registers all chart types and components used across the app in one place,
 * avoiding duplicate echarts.use() calls scattered across chart files.
 */
import * as echarts from 'echarts/core';

// -- Charts ---------------------------------------------------
import { BarChart } from 'echarts/charts';
import { LineChart } from 'echarts/charts';
import { RadarChart } from 'echarts/charts';
import { GraphChart } from 'echarts/charts';

// -- Components -----------------------------------------------
import { TooltipComponent } from 'echarts/components';
import { GridComponent } from 'echarts/components';
import { LegendComponent } from 'echarts/components';

// -- Renderer -------------------------------------------------
import { CanvasRenderer } from 'echarts/renderers';

// -- Register all at once -------------------------------------
echarts.use([
    BarChart,
    LineChart,
    RadarChart,
    GraphChart,
    TooltipComponent,
    GridComponent,
    LegendComponent,
    CanvasRenderer,
]);

export default echarts;
