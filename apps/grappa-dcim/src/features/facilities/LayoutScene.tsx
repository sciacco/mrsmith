import { useEffect, useRef, useState } from 'react';
import * as THREE from 'three';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';
import type { Islet, Position } from '../../api/types';
import styles from './workspace.module.css';

export type LayoutSelection =
  | { type: 'islet'; id: number }
  | { type: 'position'; id: number }
  | null;

interface LayoutSceneProps {
  islets: Islet[];
  positions: Position[];
  selected: LayoutSelection;
  onSelect: (selection: LayoutSelection) => void;
}

type Bounds = {
  centerX: number;
  centerZ: number;
  width: number;
  depth: number;
};

const EMPTY_BOUNDS: Bounds = {
  centerX: 0,
  centerZ: 0,
  width: 10,
  depth: 10,
};

const STATUS_COLORS = {
  free: 0x10b981,
  occupied: 0x4f46e5,
  reserved: 0xf59e0b,
  unknown: 0x64748b,
  selected: 0x0ea5e9,
};

function normalizeStatus(status: string) {
  const value = status.toLowerCase();
  if (value === 'free') return 'free';
  if (value === 'occupied') return 'occupied';
  if (value === 'reserved') return 'reserved';
  return 'unknown';
}

function positionSort(a: Position, b: Position) {
  return a.num - b.num || a.id - b.id;
}

function isHalfRack(position: Position) {
  return position.rackType?.toLowerCase() === 'half' || position.type.toLowerCase() === 'half';
}

function makeMaterial(color: number, selected = false) {
  return new THREE.MeshStandardMaterial({
    color,
    roughness: 0.58,
    metalness: 0.18,
    emissive: selected ? color : 0x000000,
    emissiveIntensity: selected ? 0.14 : 0,
  });
}

function makeLabelSprite(text: string, width = 256) {
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = 96;
  const context = canvas.getContext('2d');
  if (!context) return null;
  context.clearRect(0, 0, canvas.width, canvas.height);
  context.fillStyle = 'rgba(255, 255, 255, 0.88)';
  context.strokeStyle = 'rgba(226, 232, 240, 0.95)';
  context.lineWidth = 2;
  context.beginPath();
  context.roundRect(8, 18, canvas.width - 16, 52, 18);
  context.fill();
  context.stroke();
  context.fillStyle = '#0f172a';
  context.font = '700 28px DM Sans, Arial, sans-serif';
  context.textAlign = 'center';
  context.textBaseline = 'middle';
  context.fillText(text.slice(0, 18), canvas.width / 2, 44);

  const texture = new THREE.CanvasTexture(canvas);
  texture.colorSpace = THREE.SRGBColorSpace;
  const material = new THREE.SpriteMaterial({ map: texture, transparent: true });
  const sprite = new THREE.Sprite(material);
  sprite.scale.set(width / 92, 1.04, 1);
  return sprite;
}

function addIsletLabel(group: THREE.Group, islet: Islet, x: number, z: number, selected: boolean) {
  const sprite = makeLabelSprite(islet.name);
  if (!sprite) return;
  sprite.position.set(x, selected ? 1.16 : 0.94, z);
  group.add(sprite);
}

function addPositionNumber(group: THREE.Group, position: Position, x: number, z: number, selected: boolean) {
  if (position.status === 'occupied' && !selected) return;
  const sprite = makeLabelSprite(String(position.num), 128);
  if (!sprite) return;
  sprite.position.set(x, selected ? 2.8 : 0.52, z);
  sprite.scale.multiplyScalar(0.58);
  group.add(sprite);
}

function addRack(
  group: THREE.Group,
  position: Position,
  x: number,
  z: number,
  selected: boolean,
  selectable: THREE.Object3D[],
) {
  if (position.status !== 'occupied') return;

  const half = isHalfRack(position);
  const height = half ? 1.15 : 2.1;
  const y = half && position.rackPos === 'A' ? 1.72 : height / 2 + 0.12;
  const color = selected ? STATUS_COLORS.selected : STATUS_COLORS.occupied;
  const rack = new THREE.Mesh(new THREE.BoxGeometry(0.82, height, 0.82), makeMaterial(color, selected));
  rack.position.set(x, y, z);
  rack.userData = { type: 'position', id: position.id };
  rack.castShadow = true;
  rack.receiveShadow = true;
  selectable.push(rack);
  group.add(rack);

  const trim = new THREE.Mesh(
    new THREE.BoxGeometry(0.88, 0.045, 0.88),
    new THREE.MeshStandardMaterial({ color: 0xffffff, roughness: 0.7, metalness: 0.08 }),
  );
  trim.position.set(x, y + height / 2 + 0.04, z);
  group.add(trim);
}

function addPositionTile(
  group: THREE.Group,
  position: Position,
  x: number,
  z: number,
  selected: boolean,
  selectable: THREE.Object3D[],
) {
  const status = normalizeStatus(position.status);
  const color = selected ? STATUS_COLORS.selected : STATUS_COLORS[status];
  const tile = new THREE.Mesh(new THREE.BoxGeometry(1, 0.14, 1), makeMaterial(color, selected));
  tile.position.set(x, 0.04, z);
  tile.userData = { type: 'position', id: position.id };
  tile.castShadow = true;
  tile.receiveShadow = true;
  selectable.push(tile);
  group.add(tile);

  const border = new THREE.LineSegments(
    new THREE.EdgesGeometry(new THREE.BoxGeometry(1.02, 0.16, 1.02)),
    new THREE.LineBasicMaterial({ color: selected ? STATUS_COLORS.selected : 0xdbe4ef }),
  );
  border.position.copy(tile.position);
  group.add(border);
}

function buildScene(
  scene: THREE.Scene,
  islets: Islet[],
  positions: Position[],
  selected: LayoutSelection,
  selectable: THREE.Object3D[],
) {
  const sortedIslets = [...islets].sort((a, b) => a.floor - b.floor || a.name.localeCompare(b.name));
  const positionsByIslet = new Map<number, Position[]>();
  for (const position of positions) {
    const bucket = positionsByIslet.get(position.isletId) ?? [];
    bucket.push(position);
    positionsByIslet.set(position.isletId, bucket);
  }

  const layoutGroup = new THREE.Group();
  scene.add(layoutGroup);

  const allX: number[] = [];
  const allZ: number[] = [];
  const isletsPerRow = 2;
  const isletGapX = 7.2;
  const isletGapZ = 5.8;

  sortedIslets.forEach((islet, index) => {
    const isletPositions = [...(positionsByIslet.get(islet.id) ?? [])].sort(positionSort);
    const count = Math.max(isletPositions.length, islet.rackNum, 1);
    const columns = Math.max(2, Math.ceil(Math.sqrt(count * 1.35)));
    const rows = Math.max(1, Math.ceil(count / columns));
    const originX = (index % isletsPerRow) * isletGapX - ((Math.min(sortedIslets.length, isletsPerRow) - 1) * isletGapX) / 2;
    const originZ = Math.floor(index / isletsPerRow) * isletGapZ;
    const gridWidth = (columns - 1) * 1.22;
    const gridDepth = (rows - 1) * 1.22;
    const isletSelected = selected?.type === 'islet' && selected.id === islet.id;

    const floor = new THREE.Mesh(
      new THREE.BoxGeometry(gridWidth + 1.8, 0.08, gridDepth + 1.9),
      new THREE.MeshStandardMaterial({
        color: isletSelected ? 0xe0f2fe : 0xf8fafc,
        roughness: 0.76,
        metalness: 0.04,
      }),
    );
    floor.position.set(originX, -0.08, originZ);
    floor.userData = { type: 'islet', id: islet.id };
    floor.receiveShadow = true;
    selectable.push(floor);
    layoutGroup.add(floor);
    addIsletLabel(layoutGroup, islet, originX, originZ - gridDepth / 2 - 0.88, isletSelected);

    isletPositions.forEach((position, positionIndex) => {
      const column = positionIndex % columns;
      const row = Math.floor(positionIndex / columns);
      const x = originX + column * 1.22 - gridWidth / 2;
      const z = originZ + row * 1.22 - gridDepth / 2;
      const positionSelected = selected?.type === 'position' && selected.id === position.id;
      allX.push(x);
      allZ.push(z);
      addPositionTile(layoutGroup, position, x, z, positionSelected, selectable);
      addRack(layoutGroup, position, x, z, positionSelected, selectable);
      addPositionNumber(layoutGroup, position, x, z, positionSelected);
    });

    allX.push(originX - gridWidth / 2 - 1, originX + gridWidth / 2 + 1);
    allZ.push(originZ - gridDepth / 2 - 1, originZ + gridDepth / 2 + 1);
  });

  if (allX.length === 0 || allZ.length === 0) return EMPTY_BOUNDS;
  const minX = Math.min(...allX);
  const maxX = Math.max(...allX);
  const minZ = Math.min(...allZ);
  const maxZ = Math.max(...allZ);
  return {
    centerX: (minX + maxX) / 2,
    centerZ: (minZ + maxZ) / 2,
    width: Math.max(maxX - minX, 4),
    depth: Math.max(maxZ - minZ, 4),
  };
}

function disposeObject(object: THREE.Object3D) {
  object.traverse((child) => {
    const mesh = child as THREE.Mesh;
    if (mesh.geometry) mesh.geometry.dispose();
    const material = mesh.material as THREE.Material | THREE.Material[] | undefined;
    if (Array.isArray(material)) {
      material.forEach((item) => item.dispose());
    } else if (material) {
      material.dispose();
    }
  });
}

export function LayoutScene({ islets, positions, selected, onSelect }: LayoutSceneProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [webglError, setWebglError] = useState(false);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return undefined;
    const host = container;

    let frame = 0;
    let renderer: THREE.WebGLRenderer;
    const scene = new THREE.Scene();
    const selectable: THREE.Object3D[] = [];

    try {
      renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true, preserveDrawingBuffer: true });
    } catch {
      setWebglError(true);
      return undefined;
    }

    setWebglError(false);
    renderer.setClearColor(0xffffff, 0);
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.shadowMap.enabled = true;
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.domElement.className = styles.layoutCanvas ?? '';
    renderer.domElement.setAttribute('aria-label', 'Mappa 3D di isole, rack e posizioni');
    host.appendChild(renderer.domElement);

    const camera = new THREE.PerspectiveCamera(40, 1, 0.1, 120);
    const controls = new OrbitControls(camera, renderer.domElement);
    controls.enableDamping = true;
    controls.dampingFactor = 0.08;
    controls.minDistance = 7;
    controls.maxDistance = 46;
    controls.maxPolarAngle = Math.PI * 0.47;

    scene.fog = new THREE.Fog(0xf8fafc, 24, 74);
    scene.add(new THREE.AmbientLight(0xffffff, 1.2));
    const keyLight = new THREE.DirectionalLight(0xffffff, 2.2);
    keyLight.position.set(6, 12, 8);
    keyLight.castShadow = true;
    scene.add(keyLight);
    const fillLight = new THREE.DirectionalLight(0x99f6e4, 0.7);
    fillLight.position.set(-8, 6, -6);
    scene.add(fillLight);

    const grid = new THREE.GridHelper(44, 44, 0xd8e2ef, 0xecf1f7);
    grid.position.y = -0.16;
    scene.add(grid);

    const bounds = buildScene(scene, islets, positions, selected, selectable);
    const spread = Math.max(bounds.width, bounds.depth, 8);
    camera.position.set(bounds.centerX + spread * 0.62, spread * 0.78, bounds.centerZ + spread * 0.92);
    controls.target.set(bounds.centerX, 0.35, bounds.centerZ);
    controls.update();

    const raycaster = new THREE.Raycaster();
    const pointer = new THREE.Vector2();
    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

    function resize() {
      const { width, height } = host.getBoundingClientRect();
      const nextWidth = Math.max(320, width);
      const nextHeight = Math.max(360, height);
      renderer.setSize(nextWidth, nextHeight, false);
      camera.aspect = nextWidth / nextHeight;
      camera.updateProjectionMatrix();
    }

    function handleClick(event: MouseEvent) {
      const rect = renderer.domElement.getBoundingClientRect();
      pointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
      pointer.y = -((event.clientY - rect.top) / rect.height) * 2 + 1;
      raycaster.setFromCamera(pointer, camera);
      const hit = raycaster.intersectObjects(selectable, true)[0]?.object;
      const data = hit?.userData as { type?: string; id?: number } | undefined;
      if (data?.type === 'position' && typeof data.id === 'number') {
        onSelect({ type: 'position', id: data.id });
        return;
      }
      if (data?.type === 'islet' && typeof data.id === 'number') {
        onSelect({ type: 'islet', id: data.id });
      }
    }

    const observer = new ResizeObserver(resize);
    observer.observe(host);
    renderer.domElement.addEventListener('click', handleClick);
    resize();

    function animate() {
      if (!reducedMotion) {
        const t = performance.now() / 1000;
        keyLight.position.x = 6 + Math.sin(t * 0.55) * 1.2;
      }
      controls.update();
      renderer.render(scene, camera);
      frame = window.requestAnimationFrame(animate);
    }

    animate();

    return () => {
      window.cancelAnimationFrame(frame);
      observer.disconnect();
      renderer.domElement.removeEventListener('click', handleClick);
      controls.dispose();
      disposeObject(scene);
      renderer.dispose();
      renderer.domElement.remove();
    };
  }, [islets, onSelect, positions, selected]);

  if (webglError) {
    return (
      <div className={styles.layoutSceneFallback}>
        <h3 className={styles.emptyTitle}>Vista 3D non disponibile</h3>
        <p className={styles.emptyText}>Il browser non ha inizializzato WebGL. Usa la selezione rapida e l'inspector.</p>
      </div>
    );
  }

  return (
    <div className={styles.layoutScene}>
      <div ref={containerRef} className={styles.layoutSceneViewport} />
      <div className={styles.layoutLegend} aria-hidden="true">
        <span><i className={styles.legendFree} />libera</span>
        <span><i className={styles.legendOccupied} />occupata</span>
        <span><i className={styles.legendReserved} />riservata</span>
      </div>
    </div>
  );
}
