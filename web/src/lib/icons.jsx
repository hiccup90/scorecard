import {
  Backpack,
  BookOpen,
  Calculator,
  Dumbbell,
  FileText,
  Home,
  Languages,
  Mic,
  Music2,
  NotebookPen,
  PenLine,
  Star,
} from 'lucide-react';

export function iconNode(name) {
  const map = {
    book: <BookOpen />,
    read: <BookOpen />,
    pen: <PenLine />,
    math: <Calculator />,
    note: <NotebookPen />,
    voice: <Mic />,
    letters: <Languages />,
    run: <Dumbbell />,
    bag: <Backpack />,
    home: <Home />,
    music: <Music2 />,
    star: <Star />,
  };
  return map[name] || (name ? <FileText /> : <Star />);
}

export function modeLabel(mode) {
  return mode === 'quality' ? '质量审核' : mode === 'duration' ? '按时长' : '固定分';
}
