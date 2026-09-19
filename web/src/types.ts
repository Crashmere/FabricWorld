export interface Piece {
  width: string;
  length: string;
  unit: "cm" | "m";
  count: number;
  irregular: boolean;
  note: string;
  widthMM?: number | null;
  lengthMM?: number | null;
}
export interface Photo {
  id: string;
  width: number;
  height: number;
  bytes: number;
  createdAt: string;
}
export interface Fabric {
  id: string;
  revision: number;
  name: string;
  materials: string[];
  materialPercentages?: Record<string, string>;
  composition: string;
  color: string;
  tags: string[];
  status: "unused" | "using" | "used";
  location: string;
  purchaseDate: string;
  shop: string;
  price: string;
  priceCents?: number | null;
  notes: string;
  pieces: Piece[];
  photoIds: string[];
  photos: Photo[];
  createdAt: string;
  updatedAt: string;
  deletedAt: string | null;
}
export interface FabricList {
  items: Fabric[];
  total: number;
  offset: number;
  stats: Record<string, number>;
}
export interface Change {
  revision: number;
  action: string;
  at: string;
  before: Fabric | null;
  after: Fabric | null;
}
export const statuses = { unused: "未使用", using: "使用中", used: "已用完" };
export const materials = [
  "棉",
  "麻",
  "羊毛",
  "丝",
  "涤纶",
  "粘胶",
  "锦纶",
  "氨纶",
];
export const newPiece = (): Piece => ({
  width: "",
  length: "",
  unit: "cm",
  count: 1,
  irregular: false,
  note: "",
});
export const newFabric = (): Fabric => ({
  id: "",
  revision: 0,
  name: "",
  materials: [],
  materialPercentages: {},
  composition: "",
  color: "",
  tags: [],
  status: "unused",
  location: "",
  purchaseDate: "",
  shop: "",
  price: "",
  notes: "",
  pieces: [newPiece()],
  photoIds: [],
  photos: [],
  createdAt: "",
  updatedAt: "",
  deletedAt: null,
});
export function dimensions(p: Piece) {
  return (
    (p.width || "待测") +
    " × " +
    (p.length || "待测") +
    " " +
    p.unit +
    (p.count > 1 ? " · " + p.count + " 片" : "") +
    (p.irregular ? " · 不规则" : "")
  );
}
export function materialText(f: Pick<Fabric, "materials" | "materialPercentages">) {
  return f.materials.map(material => {
    const value = f.materials.length === 1 ? "100" : f.materialPercentages?.[material];
    return value ? material + " " + value + "%" : material;
  }).join(" / ");
}
export function dateText(v: string) {
  return v
    ? new Date(v).toLocaleString("zh-CN", {
        year: "numeric",
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      })
    : "";
}
