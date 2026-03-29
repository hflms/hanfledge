import asyncio
from playwright.async_api import async_playwright
import time

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        # Setup context with a mock token and mock user data to bypass login API calls
        context = await browser.new_context(
            viewport={'width': 1280, 'height': 720},
            record_video_dir="verification/videos"
        )

        # Inject localStorage before navigation
        await context.add_init_script("""
            window.localStorage.setItem('auth-token', 'mock-token');
        """)

        page = await context.new_page()

        # Intercept API calls to mock auth and settings data
        await page.route("**/api/v1/auth/me", lambda route: route.fulfill(
            status=200,
            json={
                "id": "u-123",
                "phone": "13800138000",
                "name": "Teacher Mock",
                "school_roles": [{"role": {"name": "TEACHER"}}]
            }
        ))

        # Mock initial settings response
        await page.route("**/api/v1/admin/settings", lambda route: route.fulfill(
            status=200,
            json={
                "llm.provider": "doubao",
                "llm.dashscope.api_key": "",
                "llm.dashscope.model": "",
                "llm.dashscope.compat_base_url": "",
                "llm.doubao.api_key": "test-doubao-key",
                "llm.doubao.model": "ep-xxxx",
                "llm.doubao.compat_base_url": "https://ark.cn-beijing.volces.com/api/v3",
                "llm.deepseek.api_key": "test-ds-key",
                "llm.deepseek.model": "deepseek-chat",
                "llm.deepseek.compat_base_url": "https://api.deepseek.com/v1"
            }
        ))

        print("Navigating to settings page...")
        await page.goto("http://localhost:3000/teacher/settings")
        await page.wait_for_load_state("networkidle")

        print("Waiting for select dropdown...")
        try:
            # Look for the select element
            select = page.locator("select").first
            await select.wait_for(state="visible", timeout=5000)

            # Select Doubao
            print("Selecting Doubao...")
            await select.select_option("doubao")
            await page.wait_for_timeout(1000)
            await page.screenshot(path="verification/screenshots/doubao_settings.png")

            # Select Deepseek
            print("Selecting Deepseek...")
            await select.select_option("deepseek")
            await page.wait_for_timeout(1000)
            await page.screenshot(path="verification/screenshots/deepseek_settings.png")

            # Select OpenRouter
            print("Selecting OpenRouter...")
            await select.select_option("openrouter")
            await page.wait_for_timeout(1000)
            await page.screenshot(path="verification/screenshots/openrouter_settings.png")

        except Exception as e:
            print(f"Error interacting with page: {e}")
            await page.screenshot(path="verification/screenshots/error_state.png")

        await context.close()
        await browser.close()
        print("Done!")

if __name__ == "__main__":
    asyncio.run(main())
