import { useEffect, useRef } from 'react'
import * as THREE from 'three'

export type PageScene = 'competition' | 'traders' | 'trader'

interface Interactive3DBackgroundProps {
  currentPage?: PageScene
}

export function Interactive3DBackground({ currentPage = 'trader' }: Interactive3DBackgroundProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const sceneStateRef = useRef({
    currentPage,
    targetPage: currentPage,
  })

  useEffect(() => {
    sceneStateRef.current.targetPage = currentPage
  }, [currentPage])

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    // 1. Optimized Particle Waves Setup
    const SEPARATION = 55       // Wider spacing = fewer particles needed
    const AMOUNTX = 50          // Reduced from 65 → 50 (saves ~40% vertex work)
    const AMOUNTY = 50

    const scene = new THREE.Scene()
    const camera = new THREE.PerspectiveCamera(
      55,
      window.innerWidth / window.innerHeight,
      1,
      10000
    )
    camera.position.set(0, 480, 1100)
    camera.lookAt(0, 50, 0)

    const renderer = new THREE.WebGLRenderer({
      alpha: true,
      antialias: false,                   // GPU saving: AA not needed for point sprites
      powerPreference: 'low-power',       // GPU saving: prefer integrated GPU
    })
    renderer.setSize(window.innerWidth, window.innerHeight)
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 1.5))  // Cap at 1.5x (saves ~25% fill rate vs 2x)
    renderer.setClearColor(0x000000, 0)
    container.appendChild(renderer.domElement)

    // 2. Particle Grid (2,500 particles — down from 4,225)
    const numParticles = AMOUNTX * AMOUNTY
    const positions = new Float32Array(numParticles * 3)
    const colors = new Float32Array(numParticles * 3)

    const cJade = new THREE.Color(0x10B981)
    const cPlatinum = new THREE.Color(0xD1D5DB)

    let i = 0
    for (let ix = 0; ix < AMOUNTX; ix++) {
      for (let iy = 0; iy < AMOUNTY; iy++) {
        positions[i] = ix * SEPARATION - (AMOUNTX * SEPARATION) / 2
        positions[i + 1] = 0
        positions[i + 2] = iy * SEPARATION - (AMOUNTY * SEPARATION) / 2

        const distNorm = Math.sqrt(
          Math.pow(ix - AMOUNTX / 2, 2) + Math.pow(iy - AMOUNTY / 2, 2)
        ) / (AMOUNTX * 0.5)

        const c = distNorm < 0.5
          ? cJade.clone().lerp(cPlatinum, distNorm * 2.0)
          : cPlatinum

        colors[i] = c.r
        colors[i + 1] = c.g
        colors[i + 2] = c.b

        i += 3
      }
    }

    const geometry = new THREE.BufferGeometry()
    geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3))
    geometry.setAttribute('color', new THREE.BufferAttribute(colors, 3))

    // Smaller texture = less VRAM
    const canvas = document.createElement('canvas')
    canvas.width = 32           // Reduced from 64 → 32 (75% less texture memory)
    canvas.height = 32
    const ctx = canvas.getContext('2d')
    if (ctx) {
      const grad = ctx.createRadialGradient(16, 16, 0, 16, 16, 16)
      grad.addColorStop(0, 'rgba(255, 255, 255, 1)')
      grad.addColorStop(0.35, 'rgba(255, 255, 255, 0.75)')
      grad.addColorStop(0.7, 'rgba(255, 255, 255, 0.15)')
      grad.addColorStop(1, 'rgba(255, 255, 255, 0)')
      ctx.fillStyle = grad
      ctx.fillRect(0, 0, 32, 32)
    }
    const texture = new THREE.CanvasTexture(canvas)

    const material = new THREE.PointsMaterial({
      size: 7.5,
      vertexColors: true,
      map: texture,
      transparent: true,
      opacity: 0.75,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
    })

    const particles = new THREE.Points(geometry, material)
    particles.position.set(0, 0, -100)
    scene.add(particles)

    // 3. User Mouse Interactivity
    let mouseX = 0
    let mouseY = 0
    let targetMouseX = 0
    let targetMouseY = 0
    let count = 0

    //    - Throttled to 30fps during movement, drops to 15fps when idle (>3s no mouse activity)
    //    - Pauses entirely when tab is hidden (Page Visibility API)
    let animationFrameId: number
    let isVisible = true
    let lastFrameTime = 0
    let lastInteractionTime = performance.now()

    const onVisibilityChange = () => {
      isVisible = !document.hidden
      if (isVisible) {
        lastFrameTime = performance.now()
        lastInteractionTime = performance.now()
        animationFrameId = requestAnimationFrame(animate)
      }
    }
    document.addEventListener('visibilitychange', onVisibilityChange)

    const onPointerMove = (event: MouseEvent) => {
      lastInteractionTime = performance.now()
      targetMouseX = (event.clientX - window.innerWidth / 2)
      targetMouseY = (event.clientY - window.innerHeight / 2)

      document.documentElement.style.setProperty('--mouse-x', `${event.clientX}px`)
      document.documentElement.style.setProperty('--mouse-y', `${event.clientY}px`)
    }

    const onResize = () => {
      camera.aspect = window.innerWidth / window.innerHeight
      camera.updateProjectionMatrix()
      renderer.setSize(window.innerWidth, window.innerHeight)
    }

    window.addEventListener('pointermove', onPointerMove, { passive: true })
    window.addEventListener('resize', onResize)

    const animate = (now: number = 0) => {
      if (!isVisible) return   // Stop rendering when tab is hidden

      animationFrameId = requestAnimationFrame(animate)

      // Dynamic FPS: 30fps when active, 15fps when idle for > 3.5s
      const isIdle = now - lastInteractionTime > 3500
      const currentInterval = isIdle ? 1000 / 15 : 1000 / 30

      const delta = now - lastFrameTime
      if (delta < currentInterval) return
      lastFrameTime = now - (delta % currentInterval)

      // Smooth camera parallax
      mouseX += (targetMouseX - mouseX) * 0.05
      mouseY += (targetMouseY - mouseY) * 0.05

      const activePage = sceneStateRef.current.targetPage
      let targetCamY = 480
      let targetCamZ = 1100

      if (activePage === 'competition') {
        targetCamY = 600
        targetCamZ = 1250
      } else if (activePage === 'traders') {
        targetCamY = 420
        targetCamZ = 1000
      }

      camera.position.x += (mouseX * 0.8 - camera.position.x) * 0.05
      camera.position.y += (-mouseY * 0.6 + targetCamY - camera.position.y) * 0.05
      camera.position.z += (targetCamZ - camera.position.z) * 0.05
      camera.lookAt(0, 40, 0)

      // Canonical Double Sine Wave
      const positionAttr = geometry.attributes.position as THREE.BufferAttribute
      const posArray = positionAttr.array as Float32Array

      let idx = 0
      for (let ix = 0; ix < AMOUNTX; ix++) {
        for (let iy = 0; iy < AMOUNTY; iy++) {
          posArray[idx + 1] =
            Math.sin((ix + count) * 0.3) * 60 +
            Math.sin((iy + count) * 0.5) * 60

          idx += 3
        }
      }

      positionAttr.needsUpdate = true
      count += 0.06

      renderer.render(scene, camera)
    }

    animate()

    return () => {
      isVisible = false
      cancelAnimationFrame(animationFrameId)
      document.removeEventListener('visibilitychange', onVisibilityChange)
      window.removeEventListener('pointermove', onPointerMove)
      window.removeEventListener('resize', onResize)
      if (container.contains(renderer.domElement)) {
        container.removeChild(renderer.domElement)
      }
      renderer.dispose()
      geometry.dispose()
      material.dispose()
      texture.dispose()
    }
  }, [])

  return (
    <div
      ref={containerRef}
      className="fixed inset-0 pointer-events-none z-0 overflow-hidden"
      style={{
        opacity: 0.95,
        willChange: 'transform',
      }}
    />
  )
}
