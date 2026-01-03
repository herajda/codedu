"""MCP stdio server for Scratch project visual inspection.

This server exposes tools that allow an LLM to inspect sprite and stage
costumes/backdrops from an SB3 Scratch project file. The server extracts
images from the archive and returns them as base64-encoded data suitable
for vision models.
"""
from __future__ import annotations

import argparse
import base64
import io
import json
import logging
import os
import sys
import zipfile
from dataclasses import dataclass
from typing import Any, Dict, List, Optional

import anyio
from mcp import stdio_server, types
from mcp.server import Server


logger = logging.getLogger(__name__)


@dataclass
class CostumeInfo:
    """Information about a costume/backdrop."""
    name: str
    md5ext: str
    data_format: str  # svg, png, jpg, etc.
    rotation_center_x: float = 0.0
    rotation_center_y: float = 0.0


@dataclass
class TargetInfo:
    """Information about a sprite or stage."""
    name: str
    is_stage: bool
    costumes: List[CostumeInfo]
    current_costume: int = 0


class ScratchProjectAssets:
    """Manages access to assets within an SB3 file."""
    
    def __init__(self, sb3_path: str) -> None:
        self.sb3_path = sb3_path
        self.targets: List[TargetInfo] = []
        self._zip: Optional[zipfile.ZipFile] = None
        self._load_project()
    
    def _load_project(self) -> None:
        """Load project.json and extract target/costume metadata."""
        try:
            self._zip = zipfile.ZipFile(self.sb3_path, 'r')
            project_data = json.loads(self._zip.read('project.json'))
            
            for target in project_data.get('targets', []):
                costumes = []
                for costume in target.get('costumes', []):
                    costumes.append(CostumeInfo(
                        name=costume.get('name', 'unnamed'),
                        md5ext=costume.get('md5ext', ''),
                        data_format=costume.get('dataFormat', 'png'),
                        rotation_center_x=costume.get('rotationCenterX', 0),
                        rotation_center_y=costume.get('rotationCenterY', 0),
                    ))
                
                self.targets.append(TargetInfo(
                    name=target.get('name', 'unnamed'),
                    is_stage=target.get('isStage', False),
                    costumes=costumes,
                    current_costume=target.get('currentCostume', 0),
                ))
        except Exception as e:
            logger.error(f"Failed to load SB3 project: {e}")
            raise
    
    def get_stage(self) -> Optional[TargetInfo]:
        """Get the stage target."""
        for target in self.targets:
            if target.is_stage:
                return target
        return None
    
    def get_sprites(self) -> List[TargetInfo]:
        """Get all sprite targets (non-stage)."""
        return [t for t in self.targets if not t.is_stage]
    
    def get_target_by_name(self, name: str) -> Optional[TargetInfo]:
        """Get a target by name (case-insensitive)."""
        name_lower = name.lower()
        for target in self.targets:
            if target.name.lower() == name_lower:
                return target
        return None
    
    def get_costume_image(self, target_name: str, costume_name: Optional[str] = None) -> Optional[Dict[str, str]]:
        """Get a costume image as base64.
        
        Returns:
            Dict with 'data' (base64 string), 'format' (svg/png/jpg), 'mime_type'
        """
        target = self.get_target_by_name(target_name)
        if not target:
            return None
        
        if costume_name:
            costume = None
            for c in target.costumes:
                if c.name.lower() == costume_name.lower():
                    costume = c
                    break
            if not costume:
                return None
        else:
            # Use current costume
            if target.costumes and 0 <= target.current_costume < len(target.costumes):
                costume = target.costumes[target.current_costume]
            elif target.costumes:
                costume = target.costumes[0]
            else:
                return None
        
        return self._read_costume_data(costume)
    
    def _read_costume_data(self, costume: CostumeInfo) -> Optional[Dict[str, str]]:
        """Read costume data from the archive."""
        if not self._zip or not costume.md5ext:
            return None
        
        try:
            data = self._zip.read(costume.md5ext)
            b64_data = base64.b64encode(data).decode('ascii')
            
            # Determine MIME type
            fmt = costume.data_format.lower()
            if fmt == 'svg':
                mime_type = 'image/svg+xml'
            elif fmt == 'png':
                mime_type = 'image/png'
            elif fmt in ('jpg', 'jpeg'):
                mime_type = 'image/jpeg'
            elif fmt == 'gif':
                mime_type = 'image/gif'
            else:
                mime_type = 'application/octet-stream'
            
            return {
                'data': b64_data,
                'format': fmt,
                'mime_type': mime_type,
                'costume_name': costume.name,
            }
        except Exception as e:
            logger.error(f"Failed to read costume {costume.md5ext}: {e}")
            return None
    
    def close(self) -> None:
        """Close the ZIP file."""
        if self._zip:
            self._zip.close()
            self._zip = None


class ScratchMCPServer:
    """MCP server for Scratch project asset inspection."""
    
    def __init__(self, sb3_path: str) -> None:
        self.assets = ScratchProjectAssets(sb3_path)
        self.server = Server(
            "scratch-assets",
            instructions=(
                "This server provides access to visual assets (costumes and backdrops) "
                "from a Scratch project. Use these tools to inspect sprite appearances "
                "when evaluating visual criteria."
            ),
        )
        self._register_handlers()
    
    def _register_handlers(self) -> None:
        @self.server.list_tools()
        async def list_tools() -> List[types.Tool]:
            return [
                types.Tool(
                    name="list_project_assets",
                    description=(
                        "List all sprites and stage in the project with their costume names. "
                        "Use this to see what visual assets are available before requesting images."
                    ),
                    inputSchema={
                        "type": "object",
                        "properties": {},
                        "required": [],
                        "additionalProperties": False,
                    },
                ),
                types.Tool(
                    name="get_sprite_costume",
                    description=(
                        "Get the image of a sprite's costume as base64. "
                        "Use this to visually inspect what a sprite looks like. "
                        "If costume_name is not provided, returns the sprite's current/default costume."
                    ),
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "sprite_name": {
                                "type": "string",
                                "description": "Name of the sprite (case-insensitive)",
                            },
                            "costume_name": {
                                "type": "string",
                                "description": "Optional: specific costume name. If omitted, uses current costume.",
                            },
                        },
                        "required": ["sprite_name"],
                        "additionalProperties": False,
                    },
                ),
                types.Tool(
                    name="get_stage_backdrop",
                    description=(
                        "Get the image of a stage backdrop as base64. "
                        "Use this to visually inspect what the stage looks like. "
                        "If backdrop_name is not provided, returns the current/default backdrop."
                    ),
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "backdrop_name": {
                                "type": "string",
                                "description": "Optional: specific backdrop name. If omitted, uses current backdrop.",
                            },
                        },
                        "required": [],
                        "additionalProperties": False,
                    },
                ),
            ]
        
        @self.server.call_tool()
        async def call_tool(name: str, arguments: Dict[str, Any]) -> List[types.TextContent | types.ImageContent]:
            if name == "list_project_assets":
                return self._tool_list_assets()
            elif name == "get_sprite_costume":
                return self._tool_get_sprite_costume(arguments)
            elif name == "get_stage_backdrop":
                return self._tool_get_stage_backdrop(arguments)
            else:
                return [types.TextContent(type="text", text=f"Unknown tool: {name}")]
    
    def _tool_list_assets(self) -> List[types.TextContent]:
        """List all project assets."""
        result = {"stage": None, "sprites": []}
        
        stage = self.assets.get_stage()
        if stage:
            result["stage"] = {
                "name": stage.name,
                "backdrops": [c.name for c in stage.costumes],
                "current_backdrop": stage.costumes[stage.current_costume].name if stage.costumes else None,
            }
        
        for sprite in self.assets.get_sprites():
            result["sprites"].append({
                "name": sprite.name,
                "costumes": [c.name for c in sprite.costumes],
                "current_costume": sprite.costumes[sprite.current_costume].name if sprite.costumes else None,
            })
        
        return [types.TextContent(type="text", text=json.dumps(result, indent=2))]
    
    def _tool_get_sprite_costume(self, args: Dict[str, Any]) -> List[types.TextContent | types.ImageContent]:
        """Get a sprite's costume image."""
        sprite_name = args.get("sprite_name", "")
        costume_name = args.get("costume_name")
        
        target = self.assets.get_target_by_name(sprite_name)
        if not target:
            return [types.TextContent(type="text", text=f"Sprite '{sprite_name}' not found")]
        if target.is_stage:
            return [types.TextContent(type="text", text=f"'{sprite_name}' is the stage, not a sprite. Use get_stage_backdrop instead.")]
        
        image_data = self.assets.get_costume_image(sprite_name, costume_name)
        if not image_data:
            return [types.TextContent(type="text", text=f"Could not load costume for sprite '{sprite_name}'")]
        
        return [
            types.TextContent(
                type="text",
                text=f"Costume '{image_data['costume_name']}' of sprite '{sprite_name}' (format: {image_data['format']})"
            ),
            types.ImageContent(
                type="image",
                data=image_data['data'],
                mimeType=image_data['mime_type'],
            ),
        ]
    
    def _tool_get_stage_backdrop(self, args: Dict[str, Any]) -> List[types.TextContent | types.ImageContent]:
        """Get a stage backdrop image."""
        backdrop_name = args.get("backdrop_name")
        
        stage = self.assets.get_stage()
        if not stage:
            return [types.TextContent(type="text", text="No stage found in project")]
        
        image_data = self.assets.get_costume_image(stage.name, backdrop_name)
        if not image_data:
            return [types.TextContent(type="text", text="Could not load backdrop")]
        
        return [
            types.TextContent(
                type="text",
                text=f"Backdrop '{image_data['costume_name']}' (format: {image_data['format']})"
            ),
            types.ImageContent(
                type="image",
                data=image_data['data'],
                mimeType=image_data['mime_type'],
            ),
        ]
    
    async def run(self) -> None:
        """Run the MCP server."""
        try:
            async with stdio_server() as streams:
                await self.server.run(
                    streams[0],
                    streams[1],
                    self.server.create_initialization_options(),
                )
        finally:
            self.assets.close()


def main() -> None:
    parser = argparse.ArgumentParser(description="MCP server for Scratch project assets")
    parser.add_argument("--sb3-path", required=True, help="Path to the SB3 file")
    parser.add_argument("--log-level", default="INFO", help="Logging level")
    args = parser.parse_args()
    
    level = getattr(logging, args.log_level.upper(), logging.INFO)
    logging.basicConfig(
        level=level,
        format="[%(levelname)s] %(name)s: %(message)s",
        stream=sys.stderr,
        force=True,
    )
    
    if not os.path.isfile(args.sb3_path):
        print(json.dumps({"error": f"SB3 file not found: {args.sb3_path}"}), file=sys.stderr)
        sys.exit(1)
    
    server = ScratchMCPServer(args.sb3_path)
    anyio.run(server.run)


if __name__ == "__main__":
    main()
