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

    // 1. Optimized Particle Grid Setup
    const SEPARATION = 55
    const AMOUNTX = 50
    const AMOUNTY = 50
    const numParticles = AMOUNTX * AMOUNTY

    const scene = new THREE.Scene()
    const camera = new THREE.PerspectiveCamera(
      55,
      window.innerWidth / window.innerHeight,
      1,
      10000
    )
    camera.position.set(0, 480, 1100)
    camera.lookAt(0, 50, 0)

    // 2. High-Efficiency WebGL Renderer
    const renderer = new THREE.WebGLRenderer({
      alpha: true,
      antialias: false,                   // GPU saving: AA unnecessary for point sprites
      powerPreference: 'low-power',       // GPU saving: favor integrated low-power silicon
    })
    renderer.setSize(window.innerWidth, window.innerHeight)
    // Standard 1.0x pixel ratio saves ~50% GPU fill-rate on Retina / 4K displays
    renderer.setPixelRatio(1.0)
    renderer.setClearColor(0x000000, 0)
    container.appendChild(renderer.domElement)

    // 3. Grid Coordinates & Radial Color Gradient Buffer
    const positions = new Float32Array(numParticles * 3)
    const colors = new Float32Array(numParticles * 3)
    const gridIndices = new Float32Array(numParticles * 2) // [ix, iy] for GPU vertex calculation

    const cJade = new THREE.Color(0x10B981)
    const cPlatinum = new THREE.Color(0xD1D5DB)

    let pIdx = 0
    let gIdx = 0
    for (let ix = 0; ix < AMOUNTX; ix++) {
      for (let iy = 0; iy < AMOUNTY; iy++) {
        positions[pIdx] = ix * SEPARATION - (AMOUNTX * SEPARATION) / 2
        positions[pIdx + 1] = 0.0 // Base height, computed on GPU
        positions[pIdx + 2] = iy * SEPARATION - (AMOUNTY * SEPARATION) / 2

        gridIndices[gIdx] = ix
        gridIndices[gIdx + 1] = iy

        const distNorm =
          Math.sqrt(
            Math.pow(ix - AMOUNTX / 2, 2) + Math.pow(iy - AMOUNTY / 2, 2)
          ) /
          (AMOUNTX * 0.5)

        const c =
          distNorm < 0.5
            ? cJade.clone().lerp(cPlatinum, distNorm * 2.0)
            : cPlatinum

        colors[pIdx] = c.r
        colors[pIdx + 1] = c.g
        colors[pIdx + 2] = c.b

        pIdx += 3
        gIdx += 2
      }
    }

    const geometry = new THREE.BufferGeometry()
    geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3))
    geometry.setAttribute('color', new THREE.BufferAttribute(colors, 3))
    geometry.setAttribute('aGrid', new THREE.BufferAttribute(gridIndices, 2))

    // 4. Circular Particle Sprite Texture (Tiny 32x32 = 75% less texture memory)
    const canvas = document.createElement('canvas')
    canvas.width = 32
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

    // 5. Zero-CPU GPU Vertex Shader
    const customShaderMaterial = new THREE.ShaderMaterial({
      precision: 'mediump',
      uniforms: {
        uTime: { value: 0 },
        uPointTexture: { value: texture },
      },
      vertexShader: `
        precision mediump float;
        attribute vec2 aGrid;
        varying vec3 vColor;
        uniform float uTime;

        void main() {
          vColor = color;
          
          // GPU-native dual sine wave modulation
          float yOffset = sin((aGrid.x + uTime) * 0.3) * 60.0 + sin((aGrid.y + uTime) * 0.5) * 60.0;
          vec3 transformed = vec3(position.x, position.y + yOffset, position.z);

          vec4 mvPosition = modelViewMatrix * vec4(transformed, 1.0);
          gl_Position = projectionMatrix * mvPosition;

          // Point attenuation
          gl_PointSize = (260.0 / -mvPosition.z);
        }
      `,
      fragmentShader: `
        precision mediump float;
        varying vec3 vColor;
        uniform sampler2D uPointTexture;

        void main() {
          vec4 texColor = texture2D(uPointTexture, gl_PointCoord);
          if (texColor.a < 0.05) discard;
          gl_FragColor = vec4(vColor, texColor.a * 0.75);
        }
      `,
      transparent: true,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
      vertexColors: true,
    })

    const particles = new THREE.Points(geometry, customShaderMaterial)
    particles.position.set(0, 0, -100)
    scene.add(particles)

    // 6. User Pointer Tracking & Reactive Loops
    let mouseX = 0
    let mouseY = 0
    let targetMouseX = 0
    let targetMouseY = 0

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

    let lastStyleTime = 0
    const onPointerMove = (event: MouseEvent) => {
      const now = performance.now()
      const wasAsleep = now - lastInteractionTime > 10000
      lastInteractionTime = now
      targetMouseX = event.clientX - window.innerWidth / 2
      targetMouseY = event.clientY - window.innerHeight / 2

      if (wasAsleep && isVisible) {
        lastFrameTime = now
        animationFrameId = requestAnimationFrame(animate)
      }

      // Throttle CSS variable updates to max 30 times per second
      if (now - lastStyleTime > 33) {
        lastStyleTime = now
        document.documentElement.style.setProperty('--mouse-x', `${event.clientX}px`)
        document.documentElement.style.setProperty('--mouse-y', `${event.clientY}px`)
      }
    }

    const onResize = () => {
      camera.aspect = window.innerWidth / window.innerHeight
      camera.updateProjectionMatrix()
      renderer.setSize(window.innerWidth, window.innerHeight)
    }

    window.addEventListener('pointermove', onPointerMove, { passive: true })
    window.addEventListener('resize', onResize)

    // 7. Dynamic Progressive Power-Saver Loop
    let timeAccumulator = 0

    const animate = (now: number = 0) => {
      if (!isVisible) return

      // Progressive Energy Tiers:
      // - Active user movement (<3s): 30 FPS
      // - Soft Idle (3s ~ 10s): 8 FPS
      // - Complete Sleep (>10s): 0 FPS (Completely stop requesting animation frames)
      const idleTime = now - lastInteractionTime
      if (idleTime > 10000) {
        // Complete sleep: do NOT schedule next frame, zero rAF overhead!
        return
      }

      animationFrameId = requestAnimationFrame(animate)

      const currentInterval = idleTime > 3000 ? 1000 / 8 : 1000 / 30

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

      // Zero-CPU Sine calculation: offloaded entirely to GPU shader!
      timeAccumulator += 0.06
      customShaderMaterial.uniforms.uTime.value = timeAccumulator

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
      customShaderMaterial.dispose()
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
