import type { Photo } from "./types";

export interface WorkFabric {
  fabricId: string;
  name: string;
  note: string;
  available: boolean;
}
export interface Work {
  id: string;
  revision: number;
  name: string;
  category: string;
  completedDate: string;
  pattern: string;
  size: string;
  recipient: string;
  tags: string[];
  notes: string;
  fabrics: WorkFabric[];
  photoIds: string[];
  photos: Photo[];
  createdAt: string;
  updatedAt: string;
  deletedAt: string | null;
}
export interface WorkList {
  items: Work[];
  total: number;
  offset: number;
  categories: string[];
}
export interface WorkChange {
  revision: number;
  action: string;
  at: string;
  before: Work | null;
  after: Work;
}
export const workCategories = ["上衣", "裙装", "裤装", "外套", "包袋", "配饰", "家居", "童装"];
export const newWork = (): Work => ({
  id: "", revision: 0, name: "", category: "", completedDate: "", pattern: "",
  size: "", recipient: "", tags: [], notes: "", fabrics: [], photoIds: [], photos: [],
  createdAt: "", updatedAt: "", deletedAt: null,
});
