import React from 'react'

function Footer() {
    return (
        <footer className="py-8 flex flex-col items-center justify-center space-y-4 w-full">
            <a
                href="https://devrow.com.ar"
                target="_blank"
                rel="noopener noreferrer"
                className="group flex flex-col items-center transition-transform duration-300 hover:scale-105"
            >
                <div className="font-bold text-4xl tracking-tight flex items-center" style={{ fontFamily: 'Inter, sans-serif' }}>
                    <span className="text-white">Dev</span>
                    <span className="text-[#FFC107]">row</span>
                </div>
                <div className="text-[9px] sm:text-[10px] font-bold tracking-[0.2em] text-white mt-1 uppercase">
                    Software - Marketing - Social Network
                </div>
            </a>
        </footer>
    )
}

export default Footer
