// -- Editor Extensions Registry --------------------------------

export { LaTeXEditor } from './LaTeXEditor';
export { ChemistryEditor } from './ChemistryEditor';
export { GeometryCanvas } from './GeometryCanvas';
export { CodePlayground } from './CodePlayground';

export interface EditorExtensionMeta {
  id: string;
  name: string;
  icon: string;
  description: string;
}

export const EDITOR_EXTENSIONS: EditorExtensionMeta[] = [
  {
    id: 'latex-formula',
    name: 'LaTeX 公式编辑器',
    icon: '∑',
    description: '输入和渲染数学公式',
  },
  {
    id: 'chemistry-equation',
    name: '化学方程式编辑器',
    icon: '⚗',
    description: '编辑化学方程式和分子结构',
  },
  {
    id: 'geometry-canvas',
    name: '几何画板',
    icon: '△',
    description: '绘制几何图形和标注',
  },
  {
    id: 'code-playground',
    name: '代码练习场',
    icon: '⌨',
    description: '编写和运行代码片段',
  },
];
